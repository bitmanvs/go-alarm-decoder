import React from 'react';
import { StyleSheet, Text, View, Dimensions, StatusBar } from 'react-native';
import { registerForPushNotificationsAsync } from './notifications';
import { TabView, SceneMap, Route } from 'react-native-tab-view';
import {TemperatureView} from './TemperatureView';
import {AlarmView} from './AlarmView';
import {Home} from './home';
import {HOMES} from './config';
import {NetworkStatus, } from './networking';

import { ReadableStream } from "web-streams-polyfill/ponyfill";
global.ReadableStream = global.ReadableStream || ReadableStream;

const initialLayout = { width: Dimensions.get('window').width };

const LazyPlaceholder = ({route}: {route: Route}) => (
  <View style={styles.container}>
    <Text>Loading {route.title}…</Text>
  </View>
);

const _renderLazyPlaceholder = ({ route }: {route: Route}) => <LazyPlaceholder route={route} />;

function Router() {
  const [index, setIndex] = React.useState(0);

  interface OurRoute extends Route {
    home: Home;
    thermostat: boolean;
  }
  const routesConfig: OurRoute[] = [];
  for (const home of HOMES) {
    if (home.alarm) {
      const title = `${home.name}\nAlarm`;
      routesConfig.push({
        key: title,
        title: title,
        thermostat: false,
        home,
      });
    }
    if (home.thermostat) {
      const title = `${home.name}\nThermostat`;
      routesConfig.push({
        key: title,
        title: title,
        thermostat: true,
        home,
      });
    }
  }

  const [routes] = React.useState(routesConfig);
  const renderScene = ({route}: {route: OurRoute}): React.ReactNode => {
    if (route.thermostat) {
      return <TemperatureView />;
    } else {
      return <AlarmView home={route.home} />
    }
  }

  return (
    <TabView<OurRoute>
      lazy
      navigationState={{ index, routes }}
      renderLazyPlaceholder={_renderLazyPlaceholder}
      renderScene={renderScene}
      onIndexChange={setIndex}
      initialLayout={initialLayout}
    />
  );
}

export default class App extends React.Component<{}, {}> {
  componentDidMount(): void {
    registerForPushNotificationsAsync();
  }

  render() {
    return (
      <View style={styles.container}>
        <NetworkStatus />
        <Router />
      </View>
    );
  }
}

const styles = StyleSheet.create({
  container: {
    marginTop: StatusBar.currentHeight,
    flex: 1,
    backgroundColor: '#fff',
    alignItems: 'stretch',
    justifyContent: 'center',
  },
});
