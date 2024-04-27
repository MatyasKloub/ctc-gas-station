package main

import (
	"fmt"
	"math/rand"
	"os"
	"sync"
	"time"

	"gopkg.in/yaml.v2"
)

type Car struct {
	id             int
	fuelType       string
	timeArrival    time.Time
	timeTank       time.Time
	timeTanked     time.Duration
	timeTankFinish time.Time
	timePay        time.Time
	timeLeave      time.Time
}
type GasStation struct {
	fuelType string
	gasPumps int // Počet benzínových pump
	carsQ    chan Car
	timeMin  time.Duration
	timeMax  time.Duration
}

type Pokladna struct {
	pokladnaQ chan Car
	id        int
	timeMin   time.Duration
	timeMax   time.Duration
}

var autProjeto int = -1
var config Config

type Statistika struct {
	Gas       StatistikaForm
	Diesel    StatistikaForm
	LPG       StatistikaForm
	Eletric   StatistikaForm
	Registers StatistikaForm
}

type StatistikaForm struct {
	total_cars     int
	total_time     int
	max_queue_time time.Duration
	durations      []time.Duration
}

var statistika Statistika

func main() {

	loadConfig(&config)

	carChannel := make(chan Car)

	var stations []GasStation
	var dieselStations []GasStation
	var eletricStations []GasStation
	var lpgStations []GasStation
	var pokladny []Pokladna
	for i := 0; i < config.Registers.Count; i++ {
		pokladna := Pokladna{pokladnaQ: make(chan Car), id: i, timeMin: ParseDuration(config.Stations.Gas.ServeTimeMin), timeMax: ParseDuration(config.Stations.Gas.ServeTimeMax)}
		pokladny = append(pokladny, pokladna)
	}
	for i := 0; i < config.Stations.Gas.Count; i++ {
		gasStation := GasStation{gasPumps: i, fuelType: "gas", timeMin: ParseDuration(config.Stations.Gas.ServeTimeMin), timeMax: ParseDuration(config.Stations.Gas.ServeTimeMax), carsQ: make(chan Car)}
		stations = append(stations, gasStation)
	}
	for i := 0; i < config.Stations.Diesel.Count; i++ {
		dieselStation := GasStation{gasPumps: i, fuelType: "diesel", timeMin: ParseDuration(config.Stations.Diesel.ServeTimeMin), timeMax: ParseDuration(config.Stations.Diesel.ServeTimeMax), carsQ: make(chan Car)}
		dieselStations = append(dieselStations, dieselStation)
	}
	for i := 0; i < config.Stations.Electric.Count; i++ {
		eletricStation := GasStation{gasPumps: i, fuelType: "eletric", timeMin: ParseDuration(config.Stations.Electric.ServeTimeMin), timeMax: ParseDuration(config.Stations.Electric.ServeTimeMax), carsQ: make(chan Car)}
		eletricStations = append(eletricStations, eletricStation)
	}
	for i := 0; i < config.Stations.LPG.Count; i++ {
		lpgStation := GasStation{gasPumps: i, fuelType: "lpg", timeMin: ParseDuration(config.Stations.LPG.ServeTimeMin), timeMax: ParseDuration(config.Stations.LPG.ServeTimeMax), carsQ: make(chan Car)}
		lpgStations = append(lpgStations, lpgStation)
	}

	var wg sync.WaitGroup
	wg.Add(len(stations))

	go func() {
		tick := time.Tick(100)
		for i := 0; i < config.Cars.Count; i++ {

			<-tick

			car := Car{
				id:       i,
				fuelType: getRandomFuelType(),
			}

			carChannel <- car
		}

		//fmt.Println(len(carChannel))
		close(carChannel)
	}()
	var returnval chan *bool = make(chan *bool)
	for _, station := range stations {
		go Process(station, &config, &wg, pokladny, returnval)
	}
	for _, station := range dieselStations {
		go Process(station, &config, &wg, pokladny, returnval)
	}
	for _, station := range eletricStations {
		go Process(station, &config, &wg, pokladny, returnval)
	}
	for _, station := range lpgStations {
		go Process(station, &config, &wg, pokladny, returnval)
	}
	for _, pokladna := range pokladny {
		go ProcessPokladna(pokladna, &config, &wg, returnval)
	}

	// Hlavní cyklus programu
	for {

		car, ok := <-carChannel

		if !ok {

			if autProjeto == config.Cars.Count-1 {
				close(returnval)
				calculateAvg()
				break
			}

			// Pokud kanál je uzavřen, přestat s cyklem
			//break
		}

		arrivalTime := time.Now()
		car.timeArrival = arrivalTime
		if car.fuelType == "gas" {
			if len(stations) > 0 {
				stations[rand.Intn(len(stations))].carsQ <- car
			}
		} else if car.fuelType == "diesel" {
			if len(dieselStations) > 0 {
				dieselStations[rand.Intn(len(dieselStations))].carsQ <- car
			}
		} else if car.fuelType == "eletric" {
			if len(eletricStations) > 0 {
				eletricStations[rand.Intn(len(eletricStations))].carsQ <- car
			}
		} else if car.fuelType == "lpg" {
			if len(lpgStations) > 0 {
				lpgStations[rand.Intn(len(lpgStations))].carsQ <- car
			}
		}

	}

}

func ParseDuration(s string) time.Duration {
	duration, _ := time.ParseDuration(s)
	return duration
}

func Process(gasStation GasStation, config *Config, wg *sync.WaitGroup, pokladny []Pokladna, zaplaceno chan *bool) {
	defer wg.Done()

	for car := range gasStation.carsQ {
		car.timeTank = time.Now()
		sleepTime := getRandomDuration(config, gasStation.fuelType)
		//fmt.Printf("%s, číslo stojanu: %d, auto:%s,%d začátek tankování: %s, doba tankování: %s\n", gasStation.fuelType, gasStation.gasPumps, car.fuelType, car.id, car.timeTank.Format("15:04:05.000"), sleepTime)
		time.Sleep(sleepTime)
		car.timeTanked = sleepTime
		car.timeTankFinish = car.timeTank.Add(time.Duration(car.timeTanked.Milliseconds()))
		if len(pokladny) > 0 {
			pokladny[rand.Intn(len(pokladny))].pokladnaQ <- car
		}

		<-zaplaceno
	}
}

func ProcessPokladna(pokladna Pokladna, config *Config, wg *sync.WaitGroup, zaplaceno chan *bool) {
	defer wg.Done()

	for car := range pokladna.pokladnaQ {
		car.timePay = time.Now()
		sleepTime := getRandomDuration(config, "pokladna")

		time.Sleep(sleepTime)
		car.timeLeave = time.Now()
		//fmt.Printf("kasa %d, platí auto: %d, start: %s, zaplaceno v: %s\n", pokladna.id, car.id, car.timePay.Format("15:04:05.000"), car.timeLeave.Format("15:04:05.000"))
		returnval := true

		switch car.fuelType {
		case "gas":
			addToStat(car.timeTanked, time.Duration(car.timeTank.Sub(car.timeArrival).Milliseconds()), car.timePay.Sub(car.timeTankFinish), 0)
		case "diesel":
			addToStat(car.timeTanked, time.Duration(car.timeTank.Sub(car.timeArrival).Milliseconds()), car.timePay.Sub(car.timeTankFinish), 1)
		case "lpg":
			addToStat(car.timeTanked, time.Duration(car.timeTank.Sub(car.timeArrival).Milliseconds()), car.timePay.Sub(car.timeTankFinish), 2)
		case "eletric":
			addToStat(car.timeTanked, time.Duration(car.timeTank.Sub(car.timeArrival).Milliseconds()), car.timePay.Sub(car.timeTankFinish), 3)
		}

		autProjeto++
		zaplaceno <- &returnval
	}

}

// pridavani statistik
func addToStat(durationTank time.Duration, durationPay time.Duration, qTillPay time.Duration, typ int) {
	switch typ {
	case 0:
		statistika.Gas.durations = append(statistika.Gas.durations, durationTank)
		statistika.Gas.total_time = statistika.Gas.total_time + int(durationTank.Milliseconds()) + int(durationPay.Milliseconds()) + int(qTillPay.Milliseconds())
		if durationTank > statistika.Gas.max_queue_time {
			statistika.Gas.max_queue_time = durationTank
		}
		statistika.Gas.total_cars++
	case 1:
		statistika.Diesel.durations = append(statistika.Diesel.durations, durationTank)
		statistika.Diesel.total_time = statistika.Diesel.total_time + int(durationTank.Milliseconds()) + int(durationPay.Milliseconds()) + int(qTillPay.Milliseconds())
		if durationTank > statistika.Diesel.max_queue_time {
			statistika.Diesel.max_queue_time = durationTank
		}
		statistika.Diesel.total_cars++
	case 2:
		statistika.LPG.durations = append(statistika.LPG.durations, durationTank)
		statistika.LPG.total_time = statistika.LPG.total_time + int(durationTank.Milliseconds()) + int(durationPay.Milliseconds()) + int(qTillPay.Milliseconds())
		if durationTank > statistika.LPG.max_queue_time {
			statistika.LPG.max_queue_time = durationTank
		}
		statistika.LPG.total_cars++
	case 3:
		statistika.Eletric.durations = append(statistika.Eletric.durations, durationTank)
		statistika.Eletric.total_time += int(durationTank.Milliseconds()) + int(durationPay.Milliseconds()) + int(qTillPay.Milliseconds())
		if durationTank > statistika.Eletric.max_queue_time {
			statistika.Eletric.max_queue_time = durationTank
		}
		statistika.Eletric.total_cars++
	}
	statistika.Registers.durations = append(statistika.Registers.durations, qTillPay)
	statistika.Registers.total_time += int(durationTank.Milliseconds()) + int(durationPay.Milliseconds()) + int(qTillPay.Milliseconds())
	statistika.Registers.total_cars++
	if qTillPay > statistika.Registers.max_queue_time {
		statistika.Registers.max_queue_time = qTillPay
	}
}

func calculateAvg() {
	var pocet int
	total := 0
	for _, duration := range statistika.Gas.durations {
		total += int(duration.Milliseconds())
		pocet++
	}
	fmt.Printf("stations:\n   gas:\n     total_cars: %d\n     total_time: %dms\n     avg_queue_time: %dms\n     max_queue_time: %s\n", statistika.Gas.total_cars, statistika.Gas.total_time, total/pocet, statistika.Gas.max_queue_time)
	pocet = 0
	total = 0
	for _, duration := range statistika.Diesel.durations {
		total += int(duration.Milliseconds())
		pocet++
	}
	fmt.Printf("   Diesel:\n     total_cars: %d\n     total_time: %dms\n     avg_queue_time: %dms\n     max_queue_time: %s\n", statistika.Diesel.total_cars, statistika.Diesel.total_time, total/pocet, statistika.Diesel.max_queue_time)
	pocet = 0
	total = 0
	for _, duration := range statistika.LPG.durations {
		total += int(duration.Milliseconds())
		pocet++
	}
	fmt.Printf("    LPG:\n     total_cars: %d\n     total_time: %dms\n     avg_queue_time: %dms\n     max_queue_time: %s\n", statistika.LPG.total_cars, statistika.LPG.total_time, total/pocet, statistika.LPG.max_queue_time)
	pocet = 0
	total = 0
	for _, duration := range statistika.Eletric.durations {
		total += int(duration.Milliseconds())
		pocet++
	}
	fmt.Printf("   Eletric:\n     total_cars: %d\n     total_time: %dms\n     avg_queue_time: %dms\n     max_queue_time: %s\n", statistika.Eletric.total_cars, statistika.Eletric.total_time, total/pocet, statistika.Eletric.max_queue_time)
	pocet = 0
	total = 0
	for _, duration := range statistika.Registers.durations {
		total += int(duration.Milliseconds())
		pocet++
	}
	fmt.Printf("Registers:\n     total_cars: %d\n     total_time_spent_on_gas_station: %dms\n     avg_queue_time_for_registers: %dms\n     max_queue_time_for_registers: %s\n", statistika.Registers.total_cars, statistika.Registers.total_time, total/pocet, statistika.Registers.max_queue_time)

}
func loadConfig(config *Config) {
	yamlFile, err := os.ReadFile("config.yaml")
	if err != nil {
		panic(fmt.Errorf("error reading config file: %s", err))
	}

	err = yaml.Unmarshal(yamlFile, &config)
	if err != nil {
		panic(fmt.Errorf("unable to decode into struct: %s", err))
	}
}

type Config struct {
	Cars struct {
		Count          int    `yaml:"count"`
		ArrivalTimeMin string `yaml:"arrival_time_min"`
		ArrivalTimeMax string `yaml:"arrival_time_max"`
	} `yaml:"cars"`
	Stations struct {
		Gas      StationConfig `yaml:"gas"`
		Diesel   StationConfig `yaml:"diesel"`
		LPG      StationConfig `yaml:"lpg"`
		Electric StationConfig `yaml:"electric"`
	} `yaml:"stations"`
	Registers struct {
		Count         int    `yaml:"count"`
		HandleTimeMin string `yaml:"handle_time_min"`
		HandleTimeMax string `yaml:"handle_time_max"`
	} `yaml:"registers"`
}

type StationConfig struct {
	Count        int    `yaml:"count"`
	ServeTimeMin string `yaml:"serve_time_min"`
	ServeTimeMax string `yaml:"serve_time_max"`
}

func getRandomFuelType() string {
	fuelTypes := []string{"gas", "diesel", "eletric", "lpg"}
	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
	return fuelTypes[rnd.Intn(len(fuelTypes))]
}

func getRandomDuration(config *Config, carType string) time.Duration {
	var timeMin time.Duration
	var timeMax time.Duration

	switch carType {
	case "gas":
		timeMin, _ = time.ParseDuration(config.Stations.Gas.ServeTimeMin)
		timeMax, _ = time.ParseDuration(config.Stations.Gas.ServeTimeMax)
	case "diesel":
		timeMin, _ = time.ParseDuration(config.Stations.Diesel.ServeTimeMin)
		timeMax, _ = time.ParseDuration(config.Stations.Diesel.ServeTimeMax)
	case "lpg":
		timeMin, _ = time.ParseDuration(config.Stations.LPG.ServeTimeMin)
		timeMax, _ = time.ParseDuration(config.Stations.LPG.ServeTimeMax)
	case "electric":
		timeMin, _ = time.ParseDuration(config.Stations.Electric.ServeTimeMin)
		timeMax, _ = time.ParseDuration(config.Stations.Electric.ServeTimeMax)
	case "arrival":
		timeMin, _ = time.ParseDuration(config.Cars.ArrivalTimeMin)
		timeMax, _ = time.ParseDuration(config.Cars.ArrivalTimeMax)
	default:
		timeMin, _ = time.ParseDuration(config.Registers.HandleTimeMin)
		timeMax, _ = time.ParseDuration(config.Registers.HandleTimeMax)
	}

	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
	returnal := timeMin + time.Duration(rnd.Intn(int(timeMax-timeMin+1)))
	return returnal
}
