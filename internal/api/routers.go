/*
 * (C) Copyright [2021-2025] Hewlett Packard Enterprise Development LP
 *
 * Permission is hereby granted, free of charge, to any person obtaining a
 * copy of this software and associated documentation files (the "Software"),
 * to deal in the Software without restriction, including without limitation
 * the rights to use, copy, modify, merge, publish, distribute, sublicense,
 * and/or sell copies of the Software, and to permit persons to whom the
 * Software is furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included
 * in all copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL
 * THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR
 * OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE,
 * ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR
 * OTHER DEALINGS IN THE SOFTWARE.
 */

package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/Cray-HPE/hms-power-control/internal/logger"

	"github.com/gorilla/mux"
)

// Route - struct containing name,method, pattern and handlerFunction to invoke.
type Route struct {
	Name        string
	Method      string
	Pattern     string
	HandlerFunc http.HandlerFunc
}

// Routes - a collection of Route
type Routes []Route

// Logger - used for logging what methods were invoked and how long they took to complete
func Logger(inner http.Handler, name string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		inner.ServeHTTP(w, r)

		if name == "GetLiveness" || 
		   name == "GetReadiness" ||
		   name == "GetHealth" {
			logger.Log.Debugf(
				"%s %s %s %s",
				r.Method,
				r.RequestURI,
				name,
				time.Since(start),
			)
		} else {
			logger.Log.Printf(
				"%s %s %s %s",
				r.Method,
				r.RequestURI,
				name,
				time.Since(start),
			)
		}
	})
}

// NewRouter - create a new mux Router; and initializes it with the routes
func NewRouter() *mux.Router {
	router := mux.NewRouter().StrictSlash(true)
	for _, route := range routes {
		var handler http.Handler = route.HandlerFunc
		handler = Logger(handler, route.Name)

		router.
			Methods(route.Method).
			Path(route.Pattern).
			Name(route.Name).
			Handler(handler)

		// With v1
		router.
			Methods(route.Method).
			Path("/v1" + route.Pattern).
			Name(route.Name).
			Handler(handler)
	}

	// If the 'pprof' build tag is set, then we will register pprof handlers,
	// otherwise this function is stubbed and will do nothing.
	RegisterPProfHandlers(router)

	return router
}

var routes = Routes{

	Route{
		"Index",
		"GET",
		"/",
		Index,
	},
	// Transitions
	Route{
		"GetTransitions",
		strings.ToUpper("get"),
		"/transitions",
		GetTransitions,
	},
	Route{
		"CreateTransition",
		strings.ToUpper("post"),
		"/transitions",
		CreateTransition,
	},
	Route{
		"GetTransitionID",
		strings.ToUpper("get"),
		"/transitions/{transitionID}",
		GetTransitions,
	},
	Route{
		"AbortTransitionID",
		strings.ToUpper("delete"),
		"/transitions/{transitionID}",
		AbortTransitionID,
	},
	// Power Status
	Route{
		"GetPowerStatus",
		strings.ToUpper("get"),
		"/power-status",
		GetPowerStatus,
	},
	Route{
		"PostPowerStatus",
		strings.ToUpper("post"),
		"/power-status",
		PostPowerStatus,
	},
	// Power Cap
	Route{
		"SnapshotPowerCap",
		strings.ToUpper("post"),
		"/power-cap/snapshot",
		SnapshotPowerCap,
	},
	Route{
		"PatchPowerCap",
		strings.ToUpper("patch"),
		"/power-cap",
		PatchPowerCap,
	},
	Route{
		"GetPowerCap",
		strings.ToUpper("get"),
		"/power-cap",
		GetPowerCap,
	},
	Route{
		"GetPowerCapQuery",
		strings.ToUpper("get"),
		"/power-cap/{taskID}",
		GetPowerCapQuery,
	},
	Route{
		"GetLiveness",
		strings.ToUpper("get"),
		"/liveness",
		GetLiveness,
	},
	Route{
		"GetReadiness",
		strings.ToUpper("get"),
		"/readiness",
		GetReadiness,
	},
	Route{
		"GetHealth",
		strings.ToUpper("get"),
		"/health",
		GetHealth,
	},
}
