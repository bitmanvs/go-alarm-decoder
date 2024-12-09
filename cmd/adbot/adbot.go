package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"sort"
	"strconv"
	"sync"
	"time"

	alarmdecoder "github.com/d4l3k/go-alarmdecoder"

	"github.com/foomo/simplecert"
	"github.com/foomo/tlsconfig"
	"github.com/gorilla/handlers"
	"github.com/jacobsa/go-serial/serial"
	expo "github.com/oliveroneill/exponent-server-sdk-golang/sdk"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/sync/errgroup"
)

var (
	bind      = flag.String("httpsbind", ":443", "address for webserver to listen on HTTPS")
	httpBind  = flag.String("httpbind", ":80", "address for webserver to listen on HTTP")
	mock      = flag.Bool("mock", false, "whether to mock out the alarm decoder device")
	email     = flag.String("email", "rice@fn.lc", "email address associated with cert")
	domain    = flag.String("domain", "", "domain to get a SSL cert for - defaults to hostname")
	name      = flag.String("name", "", "name of the house")
	port      = flag.String("port", "/dev/ttyAMA0", "serial port")
	baudRate  = flag.Uint("baud", 115200, "baud rate of the serial port")
	secretKey = flag.String("secret", "", "shared secret with clients")
	saveFile  = flag.String("savefile", "adbot.json", "file to save state in")
)

var (
	pushNotificationsSent = promauto.NewCounter(prometheus.CounterOpts{
		Name: "push_notifications_sent_total",
		Help: "The total number sent push notifications",
	})
	eventTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "event_total",
		Help: "The total number of events",
	})
	fire = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "fire",
		Help: "whether shit is on fire",
	})
	alarmSounding = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "alarm_sounding",
		Help: "whether the alarm is sounding",
	})
	ready = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "ready",
		Help: "whether the alarm is ready",
	})
	armedAway = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "armed_away",
		Help: "whether the alarm is armed away",
	})
	armedHome = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "armed_home",
		Help: "whether the alarm is armed home",
	})
)

const (
	readyMessage = "****DISARMED****  READY TO ARM"
	maxAge       = 7 * 24 * time.Hour
)

func main() {
	flag.Parse()

	if err := run(); err != nil {
		log.Fatalf("%+v", err)
	}
}

type Event struct {
	Time time.Time
	alarmdecoder.Message
}

type state struct {
	RegistrationTokens map[string]expo.ExponentPushToken
	RecentEvents       []Event
}

func (s *state) load(path string) error {
	log.Printf("loading from %+v", path)
	defer func() {
		if s.RegistrationTokens == nil {
			s.RegistrationTokens = map[string]expo.ExponentPushToken{}
		}
	}()

	if _, err := os.Stat(path); os.IsNotExist(err) {
		*s = state{}
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := json.NewDecoder(f).Decode(s); err != nil {
		log.Printf("failed to load data, using clean state: %+v", err)
		*s = state{}
	}
	return nil
}

func (s *state) save(path string) error {
	log.Printf("saving to %+v", path)
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0755)
	if err != nil {
		return err
	}
	defer f.Close()

	return json.NewEncoder(f).Encode(*s)
}

type ADBot struct {
	push *expo.PushClient

	mu struct {
		sync.Mutex

		state

		// map from device ID to token
		nextListenerID int64
		listeners      map[int64]chan<- Event
	}
}

func run() error {
	log.SetFlags(log.Lshortfile | log.Flags())
	var b ADBot
	b.mu.listeners = map[int64]chan<- Event{}
	return b.Run()
}

func dropOldEvents(events []Event, before time.Time) []Event {
	keepIdx := sort.Search(len(events), func(i int) bool {
		return events[i].Time.After(before)
	})
	return events[keepIdx:]
}

func (b *ADBot) tokens() []expo.ExponentPushToken {
	b.mu.Lock()
	defer b.mu.Unlock()

	var tokens []expo.ExponentPushToken
	for _, token := range b.mu.RegistrationTokens {
		tokens = append(tokens, token)
	}
	return tokens
}

func isHighPriority(e Event) bool {
	return e.AlarmSounding || e.Fire
}

func shouldPush(e Event) bool {
	return e.Beeps > 0 || isHighPriority(e)
}

func (b *ADBot) sendPushNotifications(ctx context.Context, e Event) error {
	if !shouldPush(e) {
		return nil
	}

	priority := expo.DefaultPriority
	ttlSeconds := 60 // 1 minute
	channelID := "event"
	title := "Alarm Event"
	if isHighPriority(e) {
		ttlSeconds = 24 * 60 * 60 // 1 day
		priority = expo.HighPriority
		channelID = "alarm"
	}

	if e.Fire {
		title = "FIRE ALARM"
	} else if e.AlarmSounding {
		title = "ALARM"
	}

	title += " - " + *name

	ctx, cancel := context.WithTimeout(ctx, 1*time.Minute)
	defer cancel()

	var messages []expo.PushMessage
	for _, token := range b.tokens() {
		log.Printf("sending to %s", token)
		messages = append(messages, expo.PushMessage{
			To:         token,
			Title:      title,
			Body:       e.KeypadMessage,
			Priority:   priority,
			TTLSeconds: ttlSeconds,
			Sound:      "default",
			ChannelID:  channelID,
		})
		pushNotificationsSent.Inc()
	}

	if len(messages) == 0 {
		return nil
	}

	resps, err := b.push.PublishMultiple(messages)
	if err != nil {
		return err
	}
	for _, resp := range resps {
		if err := resp.ValidateResponse(); err != nil {
			if err, ok := err.(*expo.DeviceNotRegisteredError); ok {
				b.removeToken(resp.PushMessage.To)
			} else {
				return err
			}
		}
	}
	return nil
}

func jsonHandler(f func(r *http.Request) (interface{}, error)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp, err := f(r)
		if err != nil {
			http.Error(w, fmt.Sprintf("%+v", err), 500)
			return
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, fmt.Sprintf("%+v", err), 500)
			return
		}
	})
}

func enforceAuth(h http.Handler, token string) http.Handler {
	if len(token) == 0 {
		log.Fatal("must specify an secret key")
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		if header != ("Bearer " + token) {
			http.Error(w, "invalid token", http.StatusForbidden)
			return
		}

		h.ServeHTTP(w, r)
	})
}

func (b *ADBot) registerHandler(r *http.Request) (interface{}, error) {
	type registerRequest struct {
		Token            string
		InstallationID   string
		DeviceName       string
		NativeAppVersion string
	}
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return nil, err
	}

	token, err := expo.NewExponentPushToken(req.Token)
	if err != nil {
		return nil, err
	}

	b.addToken(token, req.InstallationID)

	if err := b.save(); err != nil {
		return nil, err
	}

	return nil, nil
}

func (b *ADBot) save() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if err := b.mu.state.save(*saveFile); err != nil {
		return err
	}
	return nil
}

func (b *ADBot) addToken(token expo.ExponentPushToken, installID string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.mu.RegistrationTokens[installID] = token
}

func (b *ADBot) removeToken(token expo.ExponentPushToken) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for id, target := range b.mu.RegistrationTokens {
		if token == target {
			delete(b.mu.RegistrationTokens, id)
			break
		}
	}
}

func writeEventNDJSON(w io.Writer, e Event) error {
	if err := json.NewEncoder(w).Encode(e); err != nil {
		return err
	}
	return nil
}

func (b *ADBot) alarmHandler(w http.ResponseWriter, r *http.Request) {
	c := make(chan Event, 10)
	b.mu.Lock()
	events := b.mu.RecentEvents
	id := b.mu.nextListenerID
	b.mu.nextListenerID++
	b.mu.listeners[id] = c
	b.mu.Unlock()

	defer func() {
		b.mu.Lock()
		defer b.mu.Unlock()

		delete(b.mu.listeners, id)
	}()

	const maxSend = 1000
	if len(events) > maxSend {
		events = events[len(events)-maxSend:]
	}

	for _, event := range events {
		if err := writeEventNDJSON(w, event); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
	}
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}

	for event := range c {
		if err := writeEventNDJSON(w, event); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}
}

func (b *ADBot) thermostatHandler(r *http.Request) (interface{}, error) {
	return nil, errors.Errorf("unimplemented")
}

func (b *ADBot) Run() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := b.mu.state.load(*saveFile); err != nil {
		return err
	}

	if len(*name) == 0 {
		return errors.Errorf("name must be specified")
	}

	b.push = expo.NewPushClient(nil)

	secure := http.NewServeMux()
	secure.Handle("/register", jsonHandler(b.registerHandler))
	secure.Handle("/alarm", http.HandlerFunc(b.alarmHandler))
	secure.Handle("/thermostat", jsonHandler(b.thermostatHandler))
	secure.Handle("/metrics", promhttp.Handler())

	insecure := http.NewServeMux()
	insecure.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
		w.WriteHeader(200)
	})
	insecure.Handle("/", enforceAuth(secure, *secretKey))

	handler := handlers.LoggingHandler(os.Stderr, insecure)

	if len(*domain) == 0 {
		hostname, err := os.Hostname()
		if err != nil {
			return err
		}
		*domain = hostname
	}

	eg, ctx := errgroup.WithContext(ctx)
	if !*mock {
		eg.Go(func() error {
			log.Printf("Listening on %s...", *bind)
			tlsconf := tlsconfig.NewServerTLSConfig(tlsconfig.TLSModeServerStrict)
			cfg := simplecert.Default
			cfg.Domains = []string{*domain}
			cfg.CacheDir = "simplecert"
			cfg.SSLEmail = *email
			return simplecert.ListenAndServeTLSCustom(*bind, handler, cfg, tlsconf, cancel)
		})
	}

	eg.Go(func() error {
		log.Printf("Listening on %s...", *httpBind)
		return http.ListenAndServe(*httpBind, handler)
	})

	var lastMsg string

	var ad AlarmReader
	if *mock {
		log.Printf("Using mock alarm decoder...")
		ad = mockAlarmDecoder{}
	} else {
		log.Printf("Reading from %s...", *port)
		options := serial.OpenOptions{
			PortName:        *port,
			BaudRate:        *baudRate,
			DataBits:        8,
			StopBits:        1,
			MinimumReadSize: 4,
		}
		port, err := serial.Open(options)
		if err != nil {
			return err
		}
		defer port.Close()
		ad = alarmdecoder.New(port)
	}

	eg.Go(func() error {
		for ctx.Err() == nil {
			msg, err := ad.Read()
			if err != nil {
				log.Printf("error! %+v", err)
				continue
			}
			log.Printf("%+v", msg)

			e := Event{
				Time:    time.Now(),
				Message: msg,
			}

			if msg.KeypadMessage == lastMsg {
				continue
			}
			lastMsg = msg.KeypadMessage

			if !shouldPush(e) && msg.KeypadMessage == readyMessage {
				continue
			}

			b.addEvent(e)

			if err := b.sendPushNotifications(ctx, e); err != nil {
				log.Printf("sendPushNotifications error %+v", err)
			}

			if err := b.save(); err != nil {
				log.Printf("failed to save %+v", err)
			}
		}
		return nil
	})

	return eg.Wait()
}

func btof(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

func (b *ADBot) addEvent(e Event) {
	eventTotal.Inc()
	fire.Set(btof(e.Fire))
	alarmSounding.Set(btof(e.AlarmSounding))
	ready.Set(btof(e.Ready))
	armedAway.Set(btof(e.ArmedAway))
	armedHome.Set(btof(e.ArmedHome))

	b.mu.Lock()
	defer b.mu.Unlock()

	b.mu.RecentEvents = dropOldEvents(b.mu.RecentEvents, time.Now().Add(-maxAge))
	b.mu.RecentEvents = append(b.mu.RecentEvents, e)

	for _, listener := range b.mu.listeners {
		select {
		case listener <- e:
		default:
		}
	}
}

type mockAlarmDecoder struct{}

func (mockAlarmDecoder) Read() (alarmdecoder.Message, error) {
	time.Sleep(5 * time.Second)
	return alarmdecoder.Message{
		KeypadMessage: "foo " + strconv.Itoa(rand.Intn(100)),
		//AlarmSounding: rand.Float64() < 0.1,
		//Fire:          rand.Float64() < 0.1,
	}, nil
}

type AlarmReader interface {
	Read() (alarmdecoder.Message, error)
}
