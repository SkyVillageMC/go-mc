//+build ignore

// gen_packetIDs.go generates the enumeration of packet IDs used on the wire.
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"text/template"

	"github.com/iancoleman/strcase"
)

const (
	protocolURL  = "https://raw.githubusercontent.com/PrismarineJS/minecraft-data/master/data/pc/1.16.2/protocol.json"
	packetidTmpl = `// This file is automatically generated by gen_packetIDs.go. DO NOT EDIT.

package packetid

// Login state
const (
	// Clientbound
{{range $Name, $ID := .Login.Clientbound}}	{{$Name}} = {{$ID}}
{{end}}
	// Serverbound
{{range $Name, $ID := .Login.Serverbound}}	{{$Name}} = {{$ID}}
{{end}}
)

// Ping state
const (
	// Clientbound
{{range $Name, $ID := .Status.Clientbound}}	{{$Name}} = {{$ID}}
{{end}}
	// Serverbound
{{range $Name, $ID := .Status.Serverbound}}	{{$Name}} = {{$ID}}
{{end}}
)

// Play state
const (
	// Clientbound
{{range $Name, $ID := .Play.Clientbound}}	{{$Name}} = {{$ID}}
{{end}}
	// Serverbound
{{range $Name, $ID := .Play.Serverbound}}	{{$Name}} = {{$ID}}
{{end}}
)
`
)

// unnest is a utility function to unpack a value from a nested map, given
// an arbitrary set of keys to reach through.
func unnest(input map[string]interface{}, keys ...string) (map[string]interface{}, error) {
	for _, k := range keys {
		sub, ok := input[k]
		if !ok {
			return nil, fmt.Errorf("key %q not found", k)
		}
		next, ok := sub.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("key %q was %T, expected a string map", k, sub)
		}
		input = next
	}
	return input, nil
}

type duplexMappings struct {
	Clientbound map[string]string
	Serverbound map[string]string
}

func (m *duplexMappings) EnsureUniqueNames() {
	// Assemble a slice of keys to check across both maps, because we cannot
	// mutate a map while iterating it.
	clientKeys := make([]string, 0, len(m.Clientbound))
	for k, _ := range m.Clientbound {
		clientKeys = append(clientKeys, k)
	}

	for _, k := range clientKeys {
		if _, alsoServerKey := m.Serverbound[k]; alsoServerKey {
			cVal, sVal := m.Clientbound[k], m.Serverbound[k]
			delete(m.Clientbound, k)
			delete(m.Serverbound, k)
			m.Clientbound[k+"Clientbound"] = cVal
			m.Serverbound[k+"Serverbound"] = sVal
		}
	}
}

// unpackMapping returns the set of packet IDs and their names for a given
// game state.
func unpackMapping(data map[string]interface{}, gameState string) (duplexMappings, error) {
	out := duplexMappings{
		Clientbound: make(map[string]string),
		Serverbound: make(map[string]string),
	}

	info, err := unnest(data, gameState, "toClient", "types")
	if err != nil {
		return duplexMappings{}, err
	}
	pType := info["packet"].([]interface{})[1].([]interface{})[0].(map[string]interface{})["type"]
	mappings := pType.([]interface{})[1].(map[string]interface{})["mappings"].(map[string]interface{})
	for k, v := range mappings {
		out.Clientbound[strcase.ToCamel(v.(string))] = k
	}
	info, err = unnest(data, gameState, "toServer", "types")
	if err != nil {
		return duplexMappings{}, err
	}
	pType = info["packet"].([]interface{})[1].([]interface{})[0].(map[string]interface{})["type"]
	mappings = pType.([]interface{})[1].(map[string]interface{})["mappings"].(map[string]interface{})
	for k, v := range mappings {
		out.Serverbound[strcase.ToCamel(v.(string))] = k
	}

	return out, nil
}

type protocolIDs struct {
	Login  duplexMappings
	Play   duplexMappings
	Status duplexMappings
	// Handshake state has no packets
}

func downloadInfo() (*protocolIDs, error) {
	resp, err := http.Get(protocolURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var data map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	var out protocolIDs
	if out.Login, err = unpackMapping(data, "login"); err != nil {
		return nil, fmt.Errorf("login: %v", err)
	}
	out.Login.EnsureUniqueNames()
	if out.Play, err = unpackMapping(data, "play"); err != nil {
		return nil, fmt.Errorf("play: %v", err)
	}
	out.Play.EnsureUniqueNames()
	if out.Status, err = unpackMapping(data, "status"); err != nil {
		return nil, fmt.Errorf("play: %v", err)
	}
	out.Status.EnsureUniqueNames()

	return &out, nil
}

//go:generate go run $GOFILE
//go:generate go fmt packetid.go
func main() {
	pIDs, err := downloadInfo()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	f, err := os.Create("packetid.go")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	tmpl := template.Must(template.New("packetIDs").Parse(packetidTmpl))
	if err := tmpl.Execute(f, pIDs); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
