package main

import (
	"fmt"
	"log"

	"github.com/spf13/viper"
)

func main() {
	viper.SetConfigType("yml")
	viper.AddConfigPath(".")
	viper.SetConfigName(configFile)
	configuration := new(Furyconf)
	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("Error reading config file, %s", err)
	}
	err := viper.Unmarshal(configuration)
	if err != nil {
		log.Fatalf("unable to decode into struct, %v", err)
	}
	fmt.Println(configuration)

	err = configuration.Validate()
	if err != nil {
		log.Println("ERROR VALIDATING: ", err)
	}

	err = configuration.Download()
	if err != nil {
		log.Println("ERROR DOWNLOADING: ", err)
	}
}
