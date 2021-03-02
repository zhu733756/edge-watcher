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
	"fmt"

	//"context"
	//"encoding/json"
	//"strings"

	"github.com/emicklei/go-restful"

	"net/http"
)

func EdgeNodeJoin(request *restful.Request, response *restful.Response) {
	nodeName := request.QueryParameter("node_name")
	nodeIP := request.QueryParameter("node_ip")

	cmd := fmt.Sprintf("arch=$(uname -m) curl -O https://kubeedge.pek3b.qingstor.com/$arch/keadm && chmod +x keadm && ./keadm join --kubeedge-version=1.5.0 --cloudcore-ipport=139.198.121.13:10000 --certport 10002 --quicport 10001 --tunnelport 10004 --edgenode-name %s --edgenode-ip %s --token  xxx", nodeName, nodeIP)

	response.WriteHeaderAndEntity(http.StatusOK, cmd)
}
