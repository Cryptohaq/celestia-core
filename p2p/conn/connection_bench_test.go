package conn

import (
	"bytes"
	"fmt"
	"math"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/cometbft/cometbft/libs/log"
	"github.com/stretchr/testify/require"
)

const (
	kibibyte = 1024
	mebibyte = 1024 * 1024
)

// generateAndSendMessages sends a sequence of messages to the specified multiplex connection `mc`.
// Each message has the given size and is sent at the specified rate
// `messagingRate`. This process continues for the duration `totalDuration` or
// until `totalNum` messages are sent. If `totalNum` is negative,
// messaging persists for the entire `totalDuration`.
func generateAndSendMessages(mc *MConnection,
	messagingRate time.Duration,
	totalDuration time.Duration, totalNum int, msgSize int,
	msgContnet []byte, chID byte) {
	var msg []byte
	if msgContnet == nil {
		msg = bytes.Repeat([]byte{'x'}, msgSize)
	} else {
		msg = msgContnet
	}

	// message generation interval ticker
	ticker := time.NewTicker(messagingRate)
	defer ticker.Stop()

	// timer for the total duration
	timer := time.NewTimer(totalDuration)
	defer timer.Stop()

	sentNum := 0
	// generating messages
	for {
		select {
		case <-ticker.C:
			// generate message
			if mc.Send(chID, msg) {
				sentNum++
				if totalNum > 0 && sentNum >= totalNum {
					log.TestingLogger().Info("Completed the message generation as the" +
						" total number of messages is reached")
					return
				}
			}
		case <-timer.C:
			// time's up
			log.TestingLogger().Info("Completed the message generation as the total " +
				"duration is reached")
			return
		}
	}
}

func BenchmarkMConnection(b *testing.B) {
	chID := byte(0x01)

	// Testcases 1-3 evaluate the effect of send queue capacity on message transmission delay.

	// Testcases 3-5 assess the full utilization of maximum bandwidth by
	// incrementally increasing the total load while keeping send and
	// receive rates constant.
	// The transmission time should be ~ totalMsg*msgSize/sendRate,
	// indicating that the actual sendRate is in effect and has been
	// utilized.

	// Testcases 5-7 assess if surpassing available bandwidth maintains
	// consistent transmission delay without congestion or performance
	// degradation. A uniform delay across these testcases is expected.

	tests := []struct {
		name              string
		msgSize           int           // size of each message in bytes
		totalMsg          int           // total number of messages to be sent
		messagingRate     time.Duration // rate at which messages are sent
		totalDuration     time.Duration // total duration for which messages are sent
		sendQueueCapacity int           // send queue capacity i.e., the number of messages that can be buffered
		sendRate          int64         // send rate in bytes per second
		recRate           int64         // receive rate in bytes per second
	}{
		{
			// testcase 1
			// Sends one 1KB message every 20ms, totaling 50 messages (50KB/s) per second.
			// Ideal transmission time for 50 messages is ~1 second.
			// SendQueueCapacity should not affect transmission delay.
			name: "queue capacity = 1, " +
				"total load = 50 KB, " +
				"msg rate = send rate",
			msgSize:           1 * kibibyte,
			totalMsg:          1 * 50,
			sendQueueCapacity: 1,
			messagingRate:     20 * time.Millisecond,
			totalDuration:     1 * time.Minute,
			sendRate:          50 * kibibyte,
			recRate:           50 * kibibyte,
		},
		{
			// testcase 2
			// Sends one 1KB message every 20ms, totaling 50 messages (50KB/s) per second.
			// Ideal transmission time for 50 messages is ~1 second.
			// Increasing SendQueueCapacity should not affect transmission
			// delay.
			name: "queue capacity = 50, " +
				"total load = 50 KB, " +
				"traffic rate = send rate",
			msgSize:           1 * kibibyte,
			totalMsg:          1 * 50,
			sendQueueCapacity: 50,
			messagingRate:     20 * time.Millisecond,
			totalDuration:     1 * time.Minute,
			sendRate:          50 * kibibyte,
			recRate:           50 * kibibyte,
		},
		{
			// testcase 3
			// Sends one 1KB message every 20ms, totaling 50 messages (50KB/s) per second.
			// Ideal transmission time for 50 messages is ~1 second.
			// Increasing SendQueueCapacity should not affect transmission
			// delay.
			name: "queue capacity = 100, " +
				"total load = 50 KB, " +
				"traffic rate = send rate",
			msgSize:           1 * kibibyte,
			totalMsg:          1 * 50,
			sendQueueCapacity: 100,
			messagingRate:     20 * time.Millisecond,
			totalDuration:     1 * time.Minute,
			sendRate:          50 * kibibyte,
			recRate:           50 * kibibyte,
		},
		{
			// testcase 4
			// Sends one 1KB message every 20ms, totaling 50 messages (50KB/s) per second.
			// The test runs for 100 messages, expecting a total transmission time of ~2 seconds.
			name: "queue capacity = 100, " +
				"total load = 2 * 50 KB, " +
				"traffic rate = send rate",
			msgSize:           1 * kibibyte,
			totalMsg:          2 * 50,
			sendQueueCapacity: 100,
			messagingRate:     20 * time.Millisecond,
			totalDuration:     1 * time.Minute,
			sendRate:          50 * kibibyte,
			recRate:           50 * kibibyte,
		},
		{
			// testcase 5
			// Sends one 1KB message every 20ms, totaling 50 messages (50KB/s) per second.
			// The test runs for 400 messages,
			// expecting a total transmission time of ~8 seconds.
			name: "queue capacity = 100, " +
				"total load = 8 * 50 KB, " +
				"traffic rate = send rate",
			msgSize:           1 * kibibyte,
			totalMsg:          8 * 50,
			sendQueueCapacity: 100,
			messagingRate:     20 * time.Millisecond,
			totalDuration:     1 * time.Minute,
			sendRate:          50 * kibibyte,
			recRate:           50 * kibibyte,
		},
		{
			// testcase 6
			// Sends one 1KB message every 10ms, totaling 100 messages (100KB/s) per second.
			// The test runs for 400 messages,
			// expecting a total transmission time of ~8 seconds.
			name: "queue capacity = 100, " +
				"total load = 8 * 50 KB, " +
				"traffic rate = 2 * send rate",
			msgSize:           1 * kibibyte,
			totalMsg:          8 * 50,
			sendQueueCapacity: 100,
			messagingRate:     10 * time.Millisecond,
			totalDuration:     1 * time.Minute,
			sendRate:          50 * kibibyte,
			recRate:           50 * kibibyte,
		},
		{
			// testcase 7
			// Sends one 1KB message every 2ms, totaling 500 messages (500KB/s) per second.
			// The test runs for 400 messages,
			// expecting a total transmission time of ~8 seconds.
			name: "queue capacity = 100, " +
				"total load = 8 * 50 KB, " +
				"traffic rate = 10 * send rate",
			msgSize:           1 * kibibyte,
			totalMsg:          8 * 50,
			sendQueueCapacity: 100,
			messagingRate:     2 * time.Millisecond,
			totalDuration:     1 * time.Minute,
			sendRate:          50 * kibibyte,
			recRate:           50 * kibibyte,
		},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			for n := 0; n < b.N; n++ {
				// set up two networked connections
				// server, client := NetPipe() // can alternatively use this and comment out the line below
				server, client := tcpNetPipe()
				defer server.Close()
				defer client.Close()

				// prepare callback to receive messages
				allReceived := make(chan bool)
				receivedLoad := 0 // number of messages received
				onReceive := func(chID byte, msgBytes []byte) {
					receivedLoad++
					if receivedLoad >= tt.totalMsg && tt.totalMsg > 0 {
						allReceived <- true
					}
				}

				cnfg := DefaultMConnConfig()
				cnfg.SendRate = tt.sendRate
				cnfg.RecvRate = tt.recRate
				chDescs := []*ChannelDescriptor{{ID: chID, Priority: 1,
					SendQueueCapacity: tt.sendQueueCapacity}}
				clientMconn := NewMConnectionWithConfig(client, chDescs,
					func(chID byte, msgBytes []byte) {},
					func(r interface{}) {},
					cnfg)
				serverChDescs := []*ChannelDescriptor{{ID: chID, Priority: 1,
					SendQueueCapacity: tt.sendQueueCapacity}}
				serverMconn := NewMConnectionWithConfig(server, serverChDescs,
					onReceive,
					func(r interface{}) {},
					cnfg)
				clientMconn.SetLogger(log.TestingLogger())
				serverMconn.SetLogger(log.TestingLogger())

				err := clientMconn.Start()
				require.Nil(b, err)
				defer func() {
					_ = clientMconn.Stop()
				}()
				err = serverMconn.Start()
				require.Nil(b, err)
				defer func() {
					_ = serverMconn.Stop()
				}()

				// start measuring the time from here to exclude the time
				// taken to set up the connections
				b.StartTimer()
				// start generating messages, it is a blocking call
				generateAndSendMessages(clientMconn,
					tt.messagingRate,
					tt.totalDuration,
					tt.totalMsg,
					tt.msgSize,
					nil, chID)

				// wait for all messages to be received
				<-allReceived
				b.StopTimer()
			}
		})

	}

}

// tcpNetPipe creates a pair of connected net.Conn objects that can be used in tests.
func tcpNetPipe() (net.Conn, net.Conn) {
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	var conn1 net.Conn
	var wg sync.WaitGroup
	wg.Add(1)
	go func(c *net.Conn) {
		*c, _ = ln.Accept()
		wg.Done()
	}(&conn1)

	addr := ln.Addr().String()
	conn2, _ := net.Dial("tcp", addr)

	wg.Wait()

	return conn2, conn1
}

// generateExponentialSizedMessages creates and returns a series of messages
// with sizes (in the specified unit) increasing exponentially.
// The size of each message doubles, starting from 1 up to maxSizeBytes.
// unit is expected to be a power of 2.
func generateExponentialSizedMessages(maxSizeBytes int, unit int) [][]byte {
	maxSizeToUnit := maxSizeBytes / unit
	msgs := make([][]byte, 0)

	for size := 1; size <= maxSizeToUnit; size *= 2 {
		msgs = append(msgs, bytes.Repeat([]byte{'x'}, size*unit)) // create a message of the calculated size
	}
	return msgs
}

type testCase struct {
	name              string
	msgSize           int           // size of each message in bytes
	msg               []byte        // message to be sent
	totalMsg          int           // total number of messages to be sent
	messagingRate     time.Duration // rate at which messages are sent
	totalDuration     time.Duration // total duration for which messages are sent
	sendQueueCapacity int           // send queue capacity i.e., the number of messages that can be buffered
	sendRate          int64         // send rate in bytes per second
	recRate           int64         // receive rate in bytes per second
	chID              byte          // channel ID
}

func runBenchmarkTest(b *testing.B, tt testCase) {
	b.Run(tt.name, func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			// set up two networked connections
			// server, client := NetPipe() // can alternatively use this and comment out the line below
			server, client := tcpNetPipe()
			defer server.Close()
			defer client.Close()

			// prepare callback to receive messages
			allReceived := make(chan bool)
			receivedLoad := 0 // number of messages received
			onReceive := func(chID byte, msgBytes []byte) {
				receivedLoad++
				if receivedLoad >= tt.totalMsg && tt.totalMsg > 0 {
					allReceived <- true
				}
			}

			cnfg := DefaultMConnConfig()
			cnfg.SendRate = tt.sendRate
			cnfg.RecvRate = tt.recRate
			chDescs := []*ChannelDescriptor{{ID: tt.chID, Priority: 1,
				SendQueueCapacity: tt.sendQueueCapacity}}
			clientMconn := NewMConnectionWithConfig(client, chDescs,
				func(chID byte, msgBytes []byte) {},
				func(r interface{}) {},
				cnfg)
			serverChDescs := []*ChannelDescriptor{{ID: tt.chID, Priority: 1,
				SendQueueCapacity: tt.sendQueueCapacity}}
			serverMconn := NewMConnectionWithConfig(server, serverChDescs,
				onReceive,
				func(r interface{}) {},
				cnfg)
			clientMconn.SetLogger(log.TestingLogger())
			serverMconn.SetLogger(log.TestingLogger())

			err := clientMconn.Start()
			require.Nil(b, err)
			defer func() {
				_ = clientMconn.Stop()
			}()
			err = serverMconn.Start()
			require.Nil(b, err)
			defer func() {
				_ = serverMconn.Stop()
			}()

			// start measuring the time from here to exclude the time
			// taken to set up the connections
			b.StartTimer()
			// start generating messages, it is a blocking call
			generateAndSendMessages(clientMconn,
				tt.messagingRate,
				tt.totalDuration,
				tt.totalMsg,
				tt.msgSize,
				tt.msg,
				tt.chID)

			// wait for all messages to be received
			<-allReceived
			b.StopTimer()
		}
	})
}

func BenchmarkMConnection_ScalingPayloadSizes_HighSendRate(b *testing.B) {
	// One aspect that could impact the performance of MConnection and the
	// transmission rate is the size of the messages sent over the network,
	// especially when they exceed the MConnection.MaxPacketMsgPayloadSize (
	// messages are sent in packets of maximum size MConnection.
	// MaxPacketMsgPayloadSize).
	// The test cases in this benchmark involve sending messages with sizes
	// ranging exponentially from 1KB to 8192KB (
	// the max value of 8192KB is inspired by the largest possible PFB in a
	// Celestia block with 128*128 number of 512-byte shares)
	// The bandwidth is set significantly higher than the message load to ensure
	// it does not become a limiting factor.
	// All test cases are expected to complete in less than one second,
	// indicating a healthy performance.

	squareSize := 128                              // number of shares in a row/column
	shareSize := 512                               // bytes
	maxSize := squareSize * squareSize * shareSize // bytes
	msgs := generateExponentialSizedMessages(maxSize, kibibyte)
	chID := byte(0x01)

	// create test cases for each message size
	var testCases = make([]testCase, len(msgs))
	for i, msg := range msgs {
		testCases[i] = testCase{
			name:              fmt.Sprintf("msgSize = %d KB", len(msg)/kibibyte),
			msgSize:           len(msg),
			msg:               msg,
			totalMsg:          10,
			messagingRate:     time.Millisecond,
			totalDuration:     1 * time.Minute,
			sendQueueCapacity: 100,
			sendRate:          512 * mebibyte,
			recRate:           512 * mebibyte,
			chID:              chID,
		}
	}

	for _, tt := range testCases {
		runBenchmarkTest(b, tt)
	}
}

func BenchmarkMConnection_ScalingPayloadSizes_LowSendRate(b *testing.B) {
	// This benchmark test builds upon the previous one i.e.,
	// BenchmarkMConnection_ScalingPayloadSizes_HighSendRate
	// by setting the send/and receive rates lower than the message load.
	// Test cases involve sending the same load of messages but with different message sizes.
	// Since the message load and bandwidth are consistent across all test cases,
	// they are expected to complete in the same amount of time. i.e.,
	//totalLoad/sendRate.

	maxSize := 32 * kibibyte // 32KB
	msgs := generateExponentialSizedMessages(maxSize, kibibyte)
	totalLoad := float64(maxSize)
	chID := byte(0x01)
	// create test cases for each message size
	var testCases = make([]testCase, len(msgs))
	for i, msg := range msgs {
		msgSize := len(msg)
		totalMsg := int(math.Ceil(totalLoad / float64(msgSize)))
		testCases[i] = testCase{
			name:              fmt.Sprintf("msgSize = %d KB", msgSize/kibibyte),
			msgSize:           msgSize,
			msg:               msg,
			totalMsg:          totalMsg,
			messagingRate:     time.Millisecond,
			totalDuration:     1 * time.Minute,
			sendQueueCapacity: 100,
			sendRate:          4 * kibibyte,
			recRate:           4 * kibibyte,
			chID:              chID,
		}
	}

	for _, tt := range testCases {
		runBenchmarkTest(b, tt)
	}
}

// BenchmarkMConnection_Multiple_ChannelID assesses the max bw/send rate
// utilization of MConnection when configured with multiple channel IDs.
func BenchmarkMConnection_Multiple_ChannelID(b *testing.B) {
	// These tests create two connections with two channels each, one channel having higher priority.
	// Traffic is sent from the client to the server, split between the channels with varying proportions in each test case.
	// All tests should complete in 2 seconds (calculated as totalTraffic* msgSize / sendRate),
	// demonstrating that channel count doesn't affect max bandwidth utilization.
	totalTraffic := 100
	msgSize := 1 * kibibyte
	sendRate := 50 * kibibyte
	recRate := 50 * kibibyte
	chDescs := []*ChannelDescriptor{
		{ID: 0x01, Priority: 1, SendQueueCapacity: 50},
		{ID: 0x02, Priority: 2, SendQueueCapacity: 50},
	}
	type testCase struct {
		name       string
		trafficMap map[byte]int // channel ID -> number of messages to be sent
	}
	var tests []testCase
	for i := 0.0; i < 1; i += 0.1 {
		tests = append(tests, testCase{
			name: fmt.Sprintf("2 channel IDs with traffic proportion %f %f",
				i, 1-i),
			trafficMap: map[byte]int{ // channel ID -> number of messages to be sent
				0x01: int(i * float64(totalTraffic)),
				0x02: totalTraffic - int(i*float64(totalTraffic)), // the rest of the traffic
			},
		})
	}
	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			for n := 0; n < b.N; n++ {
				// set up two networked connections
				// server, client := NetPipe() // can alternatively use this and comment out the line below
				server, client := tcpNetPipe()
				defer server.Close()
				defer client.Close()

				// prepare callback to receive messages
				allReceived := make(chan bool)
				receivedLoad := 0 // number of messages received
				onReceive := func(chID byte, msgBytes []byte) {
					receivedLoad++
					if receivedLoad >= totalTraffic {
						allReceived <- true
					}
				}

				cnfg := DefaultMConnConfig()
				cnfg.SendRate = int64(sendRate)
				cnfg.RecvRate = int64(recRate)

				// mount the channel descriptors to the connections
				clientMconn := NewMConnectionWithConfig(client, chDescs,
					func(chID byte, msgBytes []byte) {},
					func(r interface{}) {},
					cnfg)
				serverMconn := NewMConnectionWithConfig(server, chDescs,
					onReceive,
					func(r interface{}) {},
					cnfg)
				clientMconn.SetLogger(log.TestingLogger())
				serverMconn.SetLogger(log.TestingLogger())

				err := clientMconn.Start()
				require.Nil(b, err)
				defer func() {
					_ = clientMconn.Stop()
				}()
				err = serverMconn.Start()
				require.Nil(b, err)
				defer func() {
					_ = serverMconn.Stop()
				}()

				// start measuring the time from here to exclude the time
				// taken to set up the connections
				b.StartTimer()
				// start generating messages over the two channels,
				// concurrently, with the specified proportions
				for chID, trafficPortion := range tt.trafficMap {
					go generateAndSendMessages(clientMconn,
						time.Millisecond,
						1*time.Minute,
						trafficPortion,
						msgSize,
						nil,
						chID)
				}

				// wait for all messages to be received
				<-allReceived
				b.StopTimer()
			}
		})
	}
}
