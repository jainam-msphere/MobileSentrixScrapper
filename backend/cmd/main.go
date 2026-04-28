package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/fasthttp/router"
	"github.com/valyala/fasthttp"
	"scrapper.com/database"
	"scrapper.com/internals/handler"
	"scrapper.com/internals/middleware"
)

func main() {
	client := database.NewClient()
	err := client.CreateTable()
	err = client.CreateBrandTable()
	err = client.CreateDeviceTable()
	if client == nil {
		log.Fatal("Error establishing db connection")
		return
	}
	if err != nil {
		if strings.Contains(err.Error(), "Cannot create preexisting table") {
			fmt.Println("Table already exists, continuing...")
		} else {
			log.Fatal("Error creating table:", err)
			return
		}
	}

	h := &handler.HandlerDb{Db: client.Db}
	router := router.New()
	router.GET("/server:health", middleware.CORSMiddleware(h.HealthChecker))
	router.GET("/brands/{brand_name}/devices/{item_name}/sources/{source_type}", middleware.CORSMiddleware(h.GetPhoneItem))
	router.GET("/manufacturers", middleware.CORSMiddleware(h.GetBrands))
	router.GET("/manufacturers/{manufacturer_name}/devices", middleware.CORSMiddleware(h.GetDevices))
	router.PATCH("/manufacturers/{manufacturer_name}/devices", middleware.CORSMiddleware(h.UpdateDevices))

	handler := func(ctx *fasthttp.RequestCtx) {
		router.Handler(ctx)
	}
	server := fasthttp.Server{
		Handler: handler,
	}
	fmt.Println("Server running at port 8080")
	log.Fatal(server.ListenAndServe(":8080"))
}
