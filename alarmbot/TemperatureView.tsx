import React from 'react';
import { StyleSheet, Text, View, Dimensions, StatusBar } from 'react-native';
import NumericInput from 'react-native-numeric-input'
import {
  LineChart,
  BarChart,
  PieChart,
  ProgressChart,
  ContributionGraph,
  StackedBarChart
} from "react-native-chart-kit";
import { ButtonGroup, Divider } from 'react-native-elements';


const TempGroup = (props: {label: string}): React.ReactNode => {
  return (
    <View style={styles.section}>
      <Text style={styles.header}>{props.label}</Text>
      <View style={styles.row}>
        <View style={styles.input}>
          <Text>Day Temp</Text>
          <NumericInput value={72} onChange={value => console.log(value)} />
        </View>

        <View style={styles.input}>
          <Text>Night Temp</Text>
          <NumericInput value={50} onChange={value => console.log(value)} />
        </View>
      </View>
    </View>
  );
}

export const TemperatureView: React.FC<{}> = () => {
  return (
    <View style={styles.container}>
      <View style={styles.section}>
        <ButtonGroup
          selectedBackgroundColor="rgba(27, 106, 158, 0.85)"
          //onPress={this.updateIndex}
          selectedIndex={0}
          buttons={['Stay', 'Away']}
          textStyle={{textAlign: 'center', fontSize: 24}}
          selectedTextStyle={{color: '#fff'}}
          containerStyle={{borderRadius: 0, height: 100}}
          containerBorderRadius={0}
        />
      </View>

      <TempGroup label="Stay" />
      <TempGroup label="Away" />

      <Text style={styles.header}>History</Text>
      <LineChart
        data={{
          labels: ["January", "February", "March", "April", "May", "June"],
          datasets: [
            {
              data: [
                Math.random() * 100,
                Math.random() * 100,
                Math.random() * 100,
                Math.random() * 100,
                Math.random() * 100,
                Math.random() * 100
              ]
            }
          ]
        }}
        width={Dimensions.get("window").width - 32} // from react-native
        height={220}
        yAxisLabel={"$"}
        yAxisSuffix={"k"}
        chartConfig={{
          backgroundColor: "#e26a00",
          backgroundGradientFrom: "#fb8c00",
          backgroundGradientTo: "#ffa726",
          decimalPlaces: 2, // optional, defaults to 2dp
          color: (opacity = 1) => `rgba(255, 255, 255, ${opacity})`,
          labelColor: (opacity = 1) => `rgba(255, 255, 255, ${opacity})`,
          style: {
            borderRadius: 16
          },
          propsForDots: {
            r: "6",
            strokeWidth: "2",
            stroke: "#ffa726"
          }
        }}
        bezier
        style={{
          marginVertical: 8,
          borderRadius: 16
        }}
      />
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    padding: 16,
    flex: 1,
    backgroundColor: '#fff',
    alignItems: 'stretch',
    justifyContent: 'flex-start',
  },
  input: {
    padding: 10,
  },
  header: {
    fontWeight: 'bold',
    fontSize: 18,
  },
  section: {
    marginTop: 16,
    marginBottom: 16,
  },
  row: {
    flexDirection: 'row',
    justifyContent: 'space-around'
  }
});
