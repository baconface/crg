// Copyright 2015-2016 The CRG Authors (see AUTHORS file).
// All rights reserved.  Use of this source code is
// governed by a GPL-style license that can be found
// in the LICENSE file.

package websocket

type command struct {
	Action    string            `json:"action"`
	Data      []string          `json:"data"`
	Field     string            `json:"field"`
	FieldData map[string]string `json:"fieldData"`
}

type state struct {
	State map[string]*string `json:"state"`
}
