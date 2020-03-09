// Copyright 2020 The gVisor Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tcp_outside_the_window_test

import (
	"testing"
	"time"

	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/seqnum"

	"golang.org/x/sys/unix"
	tb "gvisor.dev/gvisor/test/packetimpact/testbench"
)

func TestTCPOutsideTheWindow(t *testing.T) {
	dut := tb.NewDUT(t)
	defer dut.TearDown()
	listenFd, remotePort := dut.CreateListener(unix.SOCK_STREAM, unix.IPPROTO_TCP, 1)
	defer dut.Close(listenFd)
	conn := tb.NewTCPIPv4(t, dut, tb.TCP{DstPort: &remotePort}, tb.TCP{SrcPort: &remotePort})
	defer conn.Close()
	conn.Handshake()
	acceptFd, _ := dut.Accept(listenFd)
	defer dut.Close(acceptFd)

	windowSize := seqnum.Size(*conn.SynAck.WindowSize) + 2
	conn.Send(tb.TCP{
		Flags:  tb.Uint8(header.TCPFlagAck),
		SeqNum: tb.Uint32(uint32(conn.LocalSeqNum.Add(windowSize))),
	})
	if gotAck := conn.Expect(tb.TCP{Flags: tb.Uint8(header.TCPFlagAck)}, 10*time.Second); gotAck == nil {
		t.Fatal("expected an ACK packet within ten second but got none")
	}
}
