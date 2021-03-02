/*
Copyright 2021 The KubeSphere Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package edgeservice

import (
	//"context"
	"fmt"
	"log"
	"net/http"
	//"strconv"

	"github.com/emicklei/go-restful"
	"github.com/emicklei/go-restful-openapi"
)

const MIME_MERGEPATCH = "application/merge-patch+json"

func WebService() *restful.WebService {
	restful.RegisterEntityAccessor(MIME_MERGEPATCH, restful.NewEntityAccessorJSON(restful.MIME_JSON))

	ws := new(restful.WebService)
	ws.Path("/api/v1alpha1").Consumes(restful.MIME_JSON, MIME_MERGEPATCH).Produces(restful.MIME_JSON)

	tags := []string{"Edge"}

	ws.Route(ws.GET("/join/").To(EdgeNodeJoin).
		Doc("Edge node join command").
		Param(ws.QueryParameter("node_name", "Edge node name.").DataType("string").DefaultValue("").Required(true)).
		Param(ws.QueryParameter("node_ip", "Edge node internal IP.").DataType("string").DefaultValue("").Required(true)).
		Metadata(restfulspec.KeyOpenAPITags, tags).
		Consumes(restful.MIME_JSON, MIME_MERGEPATCH).
		Produces(restful.MIME_JSON))

	return ws
}

var Container = restful.DefaultContainer

func Run() {
	Container.Add(WebService())
	enableCORS()

	apiPort := 8081
	listen := fmt.Sprintf(":%d", apiPort)

	log.Printf("Edge service running\n")

	log.Printf("Edge service serving %+v\n", http.ListenAndServe(listen, nil))
}

func enableCORS() {
	// Optionally, you may need to enable CORS for the UI to work.
	cors := restful.CrossOriginResourceSharing{
		AllowedHeaders: []string{"Content-Type", "Accept"},
		AllowedMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE"},
		CookiesAllowed: false,
		AllowedDomains: []string{"*"},
		Container:      Container}
	Container.Filter(cors.Filter)
}
