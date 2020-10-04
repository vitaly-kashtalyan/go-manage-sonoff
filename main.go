package main

import (
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
)

type devices []struct {
	DeviceId string `json:"id" binding:"required"`
	Name     string `json:"name"`
	Host     string `json:"host" binding:"required"`
}

type errorMsg struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

func init() {
	_, isPresent := os.LookupEnv("DEVICES_FILE")
	if !isPresent {
		_ = os.Setenv("DEVICES_FILE", "devices.json")
	}
	fmt.Println("DEVICES_FILE:", os.Getenv("DEVICES_FILE"))
}

func main() {
	r := gin.Default()
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": http.StatusText(http.StatusOK),
		})
	})

	r.GET("/devices", func(c *gin.Context) {
		devices, err := getDevices()
		if err != nil {
			c.JSON(http.StatusInternalServerError, errorMsg{
				Status:  http.StatusText(http.StatusInternalServerError),
				Message: err.Error(),
			})
		} else {
			c.JSON(http.StatusOK, devices)
		}
	})

	r.POST("/device/:id/*proxyPath", proxy)

	r.Run() // listen and serve on 0.0.0.0:8080 (for windows "localhost:8080")
}

func getDevices() (d devices, err error) {
	jsonFile, err := os.Open(os.Getenv("DEVICES_FILE"))
	if err != nil {
		fmt.Println("error opening file ["+os.Getenv("DEVICES_FILE")+"]:", err)
		return
	}
	defer jsonFile.Close()

	byteValue, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		fmt.Println("error reading JSON:", err)
		return
	}

	err = json.Unmarshal(byteValue, &d)
	if err != nil {
		fmt.Println("error mapping from JSON to object:", err)
		return
	}
	return
}

func proxy(c *gin.Context) {
	host, err := getHostById(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorMsg{
			Status:  http.StatusText(http.StatusInternalServerError),
			Message: err.Error(),
		})
		return
	}

	if host == "" {
		c.JSON(http.StatusBadRequest, errorMsg{
			Status:  http.StatusText(http.StatusBadRequest),
			Message: "invalid device id: " + c.Param("id"),
		})
		return
	}

	remote, err := url.Parse("http://" + host)
	if err != nil {
		fmt.Println(err)
		c.JSON(http.StatusBadRequest, errorMsg{
			Status:  http.StatusText(http.StatusBadRequest),
			Message: err.Error(),
		})
		return
	}

	proxy := httputil.NewSingleHostReverseProxy(remote)
	proxy.Director = func(req *http.Request) {
		req.Header = c.Request.Header
		req.Host = remote.Host
		req.URL.Scheme = remote.Scheme
		req.URL.Host = remote.Host
		req.URL.Path = c.Param("proxyPath")
	}
	proxy.ServeHTTP(c.Writer, c.Request)
}

func getHostById(id string) (host string, err error) {
	devices, err := getDevices()
	if err == nil {
		for _, device := range devices {
			if device.DeviceId == id {
				host = device.Host
			}
		}
	}
	return
}
