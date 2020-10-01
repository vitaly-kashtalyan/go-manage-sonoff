package main

import (
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"io/ioutil"
	"net/http"
	"os"
)

type devices []struct {
	Id     uint   `json:"id"`
	Name   string `json:"name"`
	Host   string `json:"host"`
	Enable bool   `json:"enable"`
}

func main() {
	r := gin.Default()
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": http.StatusText(http.StatusOK),
		})
	})

	r.GET("/devices", func(c *gin.Context) {
		c.JSON(http.StatusOK, getDevices())
	})

	r.Run() // listen and serve on 0.0.0.0:8080 (for windows "localhost:8080")
}

func getDevices() devices {
	jsonFile, err := os.Open("devices.json")
	if err != nil {
		fmt.Println(err)
	}
	defer jsonFile.Close()
	byteValue, _ := ioutil.ReadAll(jsonFile)

	var d devices
	_ = json.Unmarshal(byteValue, &d)
	return d
}
