package main

import (
	"fmt"
	"github.com/joho/godotenv"
	"github.com/lexkong/log"
	"os"
)

func main()  {
	err :=godotenv.Load()
	if err != nil {
		log.Fatal("Load the env error: ",err)
	}
	fmt.Println(os.Getenv("PORT"));
}
