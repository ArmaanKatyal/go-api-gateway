package main

import (
	"flag"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/labstack/gommon/log"
)

var Port int

func main() {
	flag.IntVar(&Port, "port", 3000, "Port to listen on")
	flag.Parse()

	e := echo.New()
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Logger.SetLevel(log.INFO)
	e.GET("/", func(c echo.Context) error {
		slog.Info("Logging TraceID", "traceID", c.Request().Header.Get("X-Trace-ID"))
		return c.String(http.StatusOK, "Hello World! from client")
	})
	e.GET("/health", func(c echo.Context) error {
		return c.String(http.StatusOK, "OK")
	})
	if err := e.Start(":" + fmt.Sprint(Port)); err != nil && err != http.ErrServerClosed {
		e.Logger.Fatal("shutting down the server")
	}
}
