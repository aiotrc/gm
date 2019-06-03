/*
 * +===============================================
 * | Author:        Parham Alvani <parham.alvani@gmail.com>
 * |
 * | Creation Date: 17-11-2017
 * |
 * | File Name:     main.go
 * +===============================================
 */

package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/brocaar/lorawan"
	log "github.com/sirupsen/logrus"

	"github.com/jinzhu/configor"

	"github.com/gin-gonic/gin"
)

// Config represents main configuration
var Config = struct {
	Broker struct {
		URL string `default:"127.0.0.1:1883" env:"broker_url"`
	}
	Device struct {
		// Addr string `default:"2601146f"`
		Addr string `default:"0000003a"`
		// AppSKey [16]byte `default:"[0x29, 0xCB, 0xD0, 0x5A, 0x4C, 0xB9, 0xFB, 0xC5, 0x16, 0x6A, 0x89, 0xE6, 0x71, 0xC0, 0xEF, 0xCE]"`
		AppSKey [16]byte `default:"[0x2B, 0x7E, 0x15, 0x16, 0x28, 0xAE, 0xD2, 0xA6, 0xAB, 0xF7, 0x15, 0x88, 0x09, 0xCF, 0x4F, 0x3C]"`
		// NetSKey [16]byte `default:"[0x5E, 0xD4, 0x38, 0xE5, 0xC8, 0x6E, 0xDD, 0x00, 0xCE, 0x0E, 0xD6, 0x22, 0x2A, 0x99, 0xE6, 0x84]"`
		NetSKey [16]byte `default:"[0x2B, 0x7E, 0x15, 0x16, 0x28, 0xAE, 0xD2, 0xA6, 0xAB, 0xF7, 0x15, 0x88, 0x09, 0xCF, 0x4F, 0x3C]"`
	}
}{}

// handle registers apis and create http handler
func handle() http.Handler {
	r := gin.Default()

	r.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{"error": "404 Not Found"})
	})

	r.Use(gin.ErrorLogger())

	api := r.Group("/api")
	{
		api.GET("/about", aboutHandler)
		api.POST("/decrypt", decryptHandler)
	}

	return r
}

func main() {
	fmt.Println("GM AIoTRC @ 2018")

	// Load configuration
	if err := configor.Load(&Config, "config.yml"); err != nil {
		panic(err)
	}

	srv := &http.Server{
		Addr:    ":1374",
		Handler: handle(),
	}

	go func() {
		fmt.Printf("GM Listen: %s\n", srv.Addr)
		// service connections
		if err := srv.ListenAndServe(); err != nil {
			log.Fatal("Listen Error:", err)
		}
	}()

	// Set up channel on which to send signal notifications.
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, os.Interrupt, os.Kill)

	// Wait for receiving a signal.
	<-sigc

	fmt.Println("18.20 As always ... left me alone")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Shutdown Error:", err)
	}
}

func aboutHandler(c *gin.Context) {
	c.String(http.StatusOK, "18.20 is leaving us")

}

func decryptHandler(c *gin.Context) {
	c.Header("Content-Type", "application/json")

	var json decryptReq
	if err := c.BindJSON(&json); err != nil {
		return
	}

	appSKeySlice, err := hex.DecodeString(json.AppSKey)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}
	var appSKey lorawan.AES128Key
	copy(appSKey[:], appSKeySlice[:])

	netSKeySlice, err := hex.DecodeString(json.NetSKey)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}
	var netSKey lorawan.AES128Key
	copy(netSKey[:], netSKeySlice[:])

	var phy lorawan.PHYPayload
	if err := phy.UnmarshalBinary(json.PhyPayload); err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	mac, ok := phy.MACPayload.(*lorawan.MACPayload)
	if !ok {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("*MACPayload expected"))
		return
	}

	success, err := phy.ValidateMIC(netSKey)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	if !success {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("Invalid MIC"))
		return
	}

	if err := phy.DecryptFRMPayload(appSKey); err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	data, ok := mac.FRMPayload[0].(*lorawan.DataPayload)
	if !ok {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("*DataPayload expected"))
		return
	}

	c.JSON(http.StatusOK, data.Bytes)
}
