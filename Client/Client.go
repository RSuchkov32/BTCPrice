package main

import (
	commands "BTCPrice/Client/Commands"
	"log"
)

func main() {
	commands.RootCmd.PersistentFlags().StringVar(&commands.StartTime, "start", "", "Start time for the subscription (format: 2006-01-02T15:04:05Z07:00)")
	commands.RootCmd.AddCommand(commands.UsdCmd, commands.EurCmd, commands.AllCmd)
	if err := commands.RootCmd.Execute(); err != nil {
		log.Fatalf("Error while executing root command: %v", err)
	}
}
