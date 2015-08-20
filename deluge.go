// Copyright 2013 Bruno Albuquerque (bga@bug-br.org.br).
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not
// use this file except in compliance with the License. You may obtain a copy of
// the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
// WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
// License for the specific language governing permissions and limitations under
// the License.

// Package deluge implements a Go wrapper around the Deluge Remote JSON API
// (http://deluge-torrent.org/docs/1.2/core/rpc.html#remote-api). This allows
// programmers to control Deluge (http://deluge-torrent.org) programatically
// from inside Go programs. Note this is a work in progress and not everything
// is implemented but adding extra RPC calls is trivial.
package deluge

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"sync/atomic"
)

// Deluge represents an endpoint for Deluge RPC requests.
type Deluge struct {
	url      string
	password string

	client  *http.Client
	cookies []*http.Cookie

	id uint64
}

// New instantiates a new Deluge instance and authenticates with the
// server.
func New(url, password string) (*Deluge, error) {
	d := &Deluge{
		url,
		password,
		new(http.Client),
		nil,
		0,
	}

	err := d.authLogin()
	if err != nil {
		return nil, err
	}

	return d, err
}

// CoreGetTorrentState wraps the core.get_torrent_state RPC call.
// Returns a list of torrent ids in the session.
func (d *Deluge) CoreGetTorrentState() ([]string, error) {
	response, err := d.sendJsonRequest("core.get_session_state", []interface{}{})
	if err != nil {
		return nil, err
	}

	return InterfaceToStringSlice(response["result"].([]interface{})), nil
}

// CoreGetTorrentStatus wraps the core.get_torrent_status RPC call.
// torrentId is the info hash of the torrent to retrieve status for.
// Returns the map returned by status for the torrent. TODO struct?
func (d *Deluge) CoreGetTorrentStatus(torrentId string) (map[string]interface{}, error) {
	keys := []string{}

	response, err := d.sendJsonRequest("core.get_torrent_status", []interface{}{torrentId, keys})
	if err != nil {
		return nil, err
	}

	return response["result"].(map[string]interface{}), nil
}

// CoreGetTorrentsStatus wraps the core.get_torrents_status RPC call.
// It returns the status of all torrents in the session.
// Returns the map returned by status for the torrent. TODO struct?
func (d *Deluge) CoreGetTorrentsStatus() (map[string]interface{}, error) {
	filter := map[string]interface{}{}
	keys := []string{}

	response, err := d.sendJsonRequest("core.get_torrents_status", []interface{}{filter, keys})
	if err != nil {
		return nil, err
	}

	return response["result"].(map[string]interface{}), nil
}

// CoreAddTorrentFile wraps the core.add_torrent_file RPC call. fileName is the
// name of the original torrent file. fileDump is the base64 encoded contents of
// the file and options is a map with options to be set (consult de Deluge
// Torrent documentation for a list of valid options).
func (d *Deluge) CoreAddTorrentFile(fileName, fileDump string, options map[string]interface{}) (string, error) {
	response, err := d.sendJsonRequest("core.add_torrent_file", []interface{}{fileName, fileDump, options})
	if err != nil {
		return "", err
	}

	return response["result"].(string), nil
}

// CoreAddTorrentMagnet wraps the core.add_torrent_magnet RPC call. magnetUrl is
// the Magnet URL for the torrent and options is a map with options to be set
// (consult de Deluge Torrent documentation for a list of valid options).
func (d *Deluge) CoreAddTorrentMagnet(magnetUrl string, options map[string]interface{}) (string, error) {
	response, err := d.sendJsonRequest("core.add_torrent_magnet", []interface{}{magnetUrl, options})
	if err != nil {
		return "", err
	}

	return response["result"].(string), nil
}

// CoreAddTorrentUrl wraps the core.add_torrent_url RPC call. torrentUrl is
// the URL for the torrent and options is a map with options to be set
// (consult de Deluge Torrent documentation for a list of valid options).
func (d *Deluge) CoreAddTorrentUrl(torrentUrl string, options map[string]interface{}) (string, error) {
	response, err := d.sendJsonRequest("core.add_torrent_url", []interface{}{torrentUrl, options})
	if err != nil {
		return "", err
	}

	return response["result"].(string), nil
}

// CoreRemoveTorrent wraps the core.remove_torrent RPC call. torrentId is
// the info hash for the torrent to remove and removeData is a boolean flag
// for if the data attached to the torrent should be removed.
func (d *Deluge) CoreRemoveTorrent(torrentId string, removeData bool) (bool, error) {
	response, err := d.sendJsonRequest("core.remove_torrent", []interface{}{torrentId, removeData})
	if err != nil {
		return false, err
	}

	return response["result"].(bool), nil
}

func (d *Deluge) authLogin() error {
	response, err := d.sendJsonRequest("auth.login", []interface{}{d.password})
	if err != nil {
		return err
	}

	if response["result"] != true {
		return fmt.Errorf("authetication failed")
	}

	return nil
}

func (d *Deluge) sendJsonRequest(method string, params []interface{}) (map[string]interface{}, error) {
	atomic.AddUint64(&(d.id), 1)
	data, err := json.Marshal(map[string]interface{}{
		"method": method,
		"id":     d.id,
		"params": params,
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", d.url, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	if d.cookies != nil {
		for _, cookie := range d.cookies {
			req.AddCookie(cookie)
		}
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("received non-ok status to http request : %d", resp.StatusCode)
	}

	d.cookies = resp.Cookies()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	result := make(map[string]interface{})
	err = json.Unmarshal(body, &result)
	if err != nil {
		return nil, err
	}

	if result["error"] != nil {
		return nil, fmt.Errorf("json error : %v", result["error"])
	}

	return result, err
}
