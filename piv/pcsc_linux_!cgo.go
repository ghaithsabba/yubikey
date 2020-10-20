// Copyright 2020 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// +build !cgo,linux

package piv

import (
	"fmt"
	"regexp"
	"strconv"

	pcsc "github.com/gballet/go-libpcsclite"
)

// Return codes for PCSC are different on different platforms (int vs. long).

func scCheck(rc error) error {
	if rc == nil {
		return nil
	}
	fmt.Println(rc)
	err := fmt.Sprintf("%s", regexp.MustCompile(`\d[\d\w]+`).FindString(rc.Error()))
	err2, _ := strconv.ParseInt(err, 16, 64)
	_ = err2
	return &scErr{int64(err2)}
}

func isRCNoReaders(rc pcsc.ErrorCode) bool {
	return rc == pcsc.ErrSCardNoReadersAvailable
}

const rcSuccess = pcsc.SCardSuccess

type scContext struct {
	ctx *pcsc.Client
}

func newSCContext() (ctx *scContext, err error) {
	client, err := pcsc.EstablishContext(pcsc.PCSCDSockName, pcsc.ScopeSystem)
	if err != nil {
		return ctx, fmt.Errorf("Error establishing context: %v", err)
	}
	return &scContext{ctx: client}, nil
}

func (c *scContext) Close() error {
	return scCheck(c.ctx.ReleaseContext())
}

func (c *scContext) ListReaders() (cards []string, err error) {
	cards, err = c.ctx.ListReaders()
	return cards, scCheck(err)
}

type scHandle struct {
	card      *pcsc.Card
	connected bool
}

func (c *scContext) Connect(reader string) (*scHandle, error) {
	var hh scHandle
	var err error
	hh.card, err = c.ctx.Connect(reader, pcsc.ShareExclusive, pcsc.ProtocolT1)
	if err == nil {
		hh.connected = true
	}
	return &hh, scCheck(err)
}

func (h *scHandle) Close() (err error) {
	if h.connected {
		if err = h.card.Disconnect(pcsc.LeaveCard); err == nil {
			h.connected = false
		}
	}
	return scCheck(err)
}

type scTx struct {
	card      *pcsc.Card
	connected bool
}

func (h *scHandle) Begin() (*scTx, error) {
	return &scTx{card: h.card}, nil
}

func (t *scTx) Close() (err error) {
	if t.connected {
		if err = t.card.Disconnect(pcsc.LeaveCard); err == nil {
			t.connected = false
		}
	}
	return scCheck(err)
}

func (t *scTx) transmit(req []byte) (more bool, b []byte, err error) {
	resp, t2, err := t.card.Transmit(req)
	respN := len(resp)
	sw1 := resp[respN-2]
	sw2 := resp[respN-1]
	_, _ = sw1, sw2
	if sw1 == 0x90 && sw2 == 0x00 {
		return false, resp[:respN-2], err
	} else if sw1 == 0x61 {
		return true, resp[:respN-2], nil
	}
	_ = t2
	return false, nil, &apduErr{sw1, sw2}
}
