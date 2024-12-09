import React from 'react';
import { StyleSheet, Text, View, Dimensions, StatusBar, ScrollView } from 'react-native';
import { Notifications } from 'expo';
import { EventSubscription } from 'fbemitter';
import {stream, retry} from './networking';
import {HOMES} from './config';
import {Home} from './home';
import debounce from 'lodash.debounce';
import moment from 'moment';

interface Event {
  ACPower: boolean;
  AlarmHasOccured: boolean;
  AlarmSounding: boolean;
  ArmedAway: boolean;
  ArmedHome: boolean;
  BacklightOn: boolean;
  BatteryLow: boolean;
  Beeps: number;
  ChimeEnabled: boolean;
  EntryDelayDisabled: boolean;
  Fire: boolean;
  KeypadMessage: string;
  Mode: string;
  PerimeterOnly: boolean;
  ProgrammingMode: boolean;
  RawData: string;
  Ready: boolean;
  SystemIssue: boolean;
  Time: string;
  UnparsedMessage: string;
  Zone: string;
  ZoneBypassed: boolean;
}

interface AlarmViewProps {
  home: Home;
}

interface AlarmViewState {
  events: Event[];
}

export class AlarmView extends React.Component<AlarmViewProps, AlarmViewState> {
  private listener: EventSubscription;

  constructor(props: AlarmViewProps) {
    super(props);

    this.state = {events: []};
  }

  componentDidMount(): void {
    retry(async (i: number) => {
      await this.updateEvents(i);
    });
  }

  componentWillUnmount(): void {
  }

  private async updateEvents(i: number): Promise<void> {
    const idx = i % this.props.home.endpoints.length;
    const endpoint = this.props.home.endpoints[idx];
    const es = await stream<Event>(endpoint + "/alarm");
    this.setState(() => {
      return {events: []};
    });
    let pending: Event[] = [];
    const update = debounce(() => {
      this.setState(({events}) => {
        events = pending.concat(events);
        pending = [];
        return {events};
      });
    }, 100);
    while (true) {
      const result = await es.read();
      if (result.done) {
        throw new Error("stream ended");
      }
      pending.unshift(result.value);
      update();
    }
  }

  render(): React.ReactNode {
    const {events} = this.state;

    const elems: React.ReactNode[] = [];
    let lastEventTime;
    for (const e of events) {
      const time = this.eventTime(e);
      if (time !== lastEventTime) {
        elems.push(<Text key={time} style={styles.time}>{time}</Text>);
        lastEventTime = time;
      }
      elems.push(this.renderEvent(e));
    }

    return (
      <ScrollView>
        <View style={styles.container}>
          {elems}
          <Text style={styles.time}>Event count = {events.length}</Text>
        </View>
      </ScrollView>
    );
  }

  private eventTime(e: Event): string {
    return moment(e.Time).format("dddd, MMMM Do YYYY, h:mm a");
  }

  private renderEvent(e: Event): React.ReactNode {
    return (
      <Text key={e.Time} style={[
        styles.event,
        e.Fire ? styles.fire : null,
        e.AlarmSounding ? styles.alarm : null,
      ]}>
        {e.Fire ? 'FIRE ALARM - ' : null}
        {e.AlarmSounding ? 'ALARM - ' : null}
        {e.KeypadMessage}
      </Text>
    )
  }
}

const styles = StyleSheet.create({
  container: {
    padding: 16,
    flex: 1,
    backgroundColor: '#fff',
    alignItems: 'stretch',
    justifyContent: 'flex-start',
  },
  alarm: {
    backgroundColor: '#f00',
    fontWeight: 'bold',
    color: '#fff',
  },
  fire: {
    backgroundColor: '#ffa500',
    fontWeight: 'bold',
    color: '#fff',
  },
  event: {
    marginTop: 2,
    marginBottom: 2,
    padding: 10,
    backgroundColor: "#eee",
    borderRadius: 16,
  },
  time: {
    textAlign: 'center',
    margin: 10,
    color: '#aaa',
  }
})
