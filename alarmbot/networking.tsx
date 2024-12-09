import React from 'react';
import { StyleSheet, Text, View, Dimensions, StatusBar, ScrollView, ProgressBarAndroid } from 'react-native';
import '@expo/browser-polyfill';
import fetchStream from 'fetch-readablestream';
// <reference path="ndjson.d.ts" />
import ndjsonStream from 'can-ndjson-stream';

import {Home} from './home';
import {TOKEN, HOMES} from './config';

const Authorization = {
  'Authorization': 'Bearer '+TOKEN,
};

const defaultTimeout = 10000;

function timeoutPromise<T>(ms: number, promise: Promise<T>): Promise<T> {
  return new Promise((resolve, reject) => {
    const timeoutId = setTimeout(() => {
      reject(new Error("promise timeout"))
    }, ms);
    promise.then(
      (res) => {
        clearTimeout(timeoutId);
        resolve(res);
      },
      (err) => {
        clearTimeout(timeoutId);
        reject(err);
      }
    );
  })
}

export function homeFromURL(endpoint: string): Home {
  for (const home of HOMES) {
    for (const homeendpoint of home.endpoints) {
      if (endpoint.startsWith(homeendpoint)) {
        return home
      }
    }
  }
  return null;
}

async function sleep(ms: number): Promise<void> {
  return new Promise(resolve => setTimeout(resolve, ms));
}

export async function retry<T>(f: (i: number) => Promise<T>, tries: number = 10): Promise<T> {
  let sleepMs = 1000;
  for (let i=0;i<tries;i++) {
    try {
      return await f(i);
    } catch (err) {
      console.warn("try failed", i, f, err);
      if (i === (tries-1)) {
        throw err;
      }
    }
    await sleep(sleepMs);
    sleepMs *= 2;
  }
}


interface NetworkStatusState {
  inflight: number;
  connections: {[name: string]: ConnectionState};
}

enum ConnectionState {
  CONNECTING,
  SUCCEEDED,
  FAILED,
}

interface NetworkStatusProps {}

export class NetworkStatus extends React.Component<NetworkStatusProps, NetworkStatusState> {
  static active: NetworkStatus = null;
  public static track(endpoint: string, p: Promise<Response>) {
    NetworkStatus.active.track(endpoint, p);
  }

  constructor(props: NetworkStatusProps) {
    super(props);
    this.state = {
      inflight: 0,
      connections: {},
    };
    NetworkStatus.active = this;
  }

  private track(endpoint: string, p: Promise<Response>) {
    this.addInflight(1);
    this.setConnState(endpoint, ConnectionState.CONNECTING);
    p.then(() => {
      this.setConnState(endpoint, ConnectionState.SUCCEEDED);
    }).catch((err) => {
      console.warn('request failed', endpoint, err);
      this.setConnState(endpoint, ConnectionState.FAILED);
    }).finally(() => {
      this.addInflight(-1);
    });
  }

  private addInflight(offset: number): void {
    console.log('inflight changed', offset, this.state);
    this.setState(({inflight}) => {
      return {inflight: inflight+offset}
    });
  }

  private setConnState(endpoint: string, state: ConnectionState): void {
    this.setState(({connections}) => {
      const home = homeFromURL(endpoint);
      if (!home) {
        throw Error("couldn't find home for "+endpoint);
      }
      connections[home.name] = state;
      return {connections};
    });
  }

  public render(): React.ReactNode {
    const elems: React.ReactNode[] = [];
    for (const name of Object.keys(this.state.connections)) {
      const state = this.state.connections[name];
      if (state === ConnectionState.FAILED) {
        elems.push(
          <Text key={name} style={[styles.state, styles.failed]}>
            Could not reach {name}
          </Text>
        );
      } else if (state === ConnectionState.CONNECTING) {
        elems.push(
          <Text key={name} style={styles.state}>Connecting to {name}...</Text>
        );
      }
    }
    return (
      <View>
        {elems}
        {this.renderProgress()}
      </View>
    );
  }

  private renderProgress(): React.ReactNode | null {
    if (this.state.inflight == 0) {
      return null;
    }
    return <ProgressBarAndroid
      indeterminate
      styleAttr="Horizontal" />;
  }
}

function fetchWrapperStream(endpoint: string, req: RequestInit): Promise<Response> {
  req.headers = new Headers(req.headers);
  const p = timeoutPromise(defaultTimeout, fetchStream(endpoint, req));
  NetworkStatus.track(endpoint, p);
  return p;
}

function fetchWrapper(endpoint: string, req: RequestInit): Promise<Response> {
  req.headers = new Headers(req.headers);
  const p = timeoutPromise(defaultTimeout, fetch(endpoint, req));
  NetworkStatus.track(endpoint, p);
  return p;
}

export async function post<T, K>(endpoint: string, body: T): Promise<K> {
  console.log('fetching from ', endpoint);
  const resp = await fetchWrapper(endpoint, {
    method: 'POST',
    headers: {
      ...Authorization,
      Accept: 'application/json',
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(body),
  })
  return resp.json();
}

export async function get<K>(endpoint: string): Promise<K> {
  const resp = await fetchWrapper(endpoint, {
    method: 'GET',
    headers: {
      ...Authorization,
      Accept: 'application/json',
    },
  });
  return resp.json();
}

export async function stream<K>(endpoint: string): Promise<ReadableStreamDefaultReader<K>> {
  console.log('streaming from ', endpoint);
  const resp = await fetchWrapperStream(endpoint, {
    method: 'GET',
    headers: {
      ...Authorization,
      Accept: 'application/json',
    },
  });
  return ndjsonStream<K>(resp.body).getReader();
}

const styles = StyleSheet.create({
  state: {
    padding: 5,
    textAlign: 'center',
    color: '#aaa',
  },
  failed: {
    backgroundColor: 'red',
    color: 'white',
  },
});
