// +build integration

package integrationtests

import (
	"context"
	"fmt"
	"log"
	"strings"
	"testing"

	"github.com/gofrs/uuid"
	"github.com/jackc/pgx/pgproto3"

	"github.com/onederx/bitcoin-processing/bitcoin"
	"github.com/onederx/bitcoin-processing/bitcoin/wallet"
	"github.com/onederx/bitcoin-processing/events"
	"github.com/onederx/bitcoin-processing/integrationtests/testenv"
	"github.com/onederx/bitcoin-processing/integrationtests/testenv/pgmitm"
	"github.com/onederx/bitcoin-processing/integrationtests/util"
)

type failureType int

const (
	FailureConnFailure failureType = iota
	FailureConnFailureMultiple
	FailureProcessingCrash
	FailureDBCrash
)

var failureTypes = map[failureType]string{
	FailureConnFailure:         "ConnFailure",
	FailureConnFailureMultiple: "ConnFailureMultiple",
	FailureProcessingCrash:     "ProcessingCrash",
	FailureDBCrash:             "DBCrash",
}

type failureMoment int

const (
	FailureMomentTxWrite failureMoment = iota
	FailureMomentEventWrite
	FailureMomentLastSeenBlockHashWrite
	FailureMomentHTTPSentSeqWrite
	FailureMomentCommit
)

var failureMoments = map[failureMoment]string{
	FailureMomentTxWrite:                "TxWrite",
	FailureMomentEventWrite:             "EventWrite",
	FailureMomentLastSeenBlockHashWrite: "LastSeenBlockHashWrite",
	FailureMomentHTTPSentSeqWrite:       "HTTPSentSeqWrite",
}

type queryType int

const (
	queryUndefined queryType = iota
	queryTxWrite
	queryEventWrite
	queryMetainfoWrite
)

func TestDepositReliability(t *testing.T) {
	ctx := context.Background()
	env, err := testenv.NewTestEnvironment(ctx)
	if err != nil {
		t.Fatal(err)
	}
	err = env.Start(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer env.Stop(ctx)
	env.WaitForLoad()

	eventSeq := 0

	for failure, failureName := range failureTypes {
		for moment, momentName := range failureMoments {
			runSubtest(t, failureName+"On"+momentName, func(t *testing.T) {
				eventSeq = runDepositReliabilityCase(t, env, ctx, failure, moment, eventSeq)
			})
		}
	}
}

func TestWithdrawReliability(t *testing.T) {
	ctx := context.Background()
	env, err := testenv.NewTestEnvironment(ctx)
	if err != nil {
		t.Fatal(err)
	}
	err = env.Start(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer env.Stop(ctx)
	env.WaitForLoad()

	eventSeq := 0

	withdrawFailureMoments := make(map[failureMoment]string)

	for _, m := range []failureMoment{FailureMomentTxWrite, FailureMomentEventWrite, FailureMomentHTTPSentSeqWrite} {
		withdrawFailureMoments[m] = failureMoments[m]
	}

	for failure, failureName := range failureTypes {
		for moment, momentName := range withdrawFailureMoments {
			runSubtest(t, failureName+"On"+momentName, func(t *testing.T) {
				eventSeq = runWithdrawReliabilityCase(t, env, ctx, failure, moment, eventSeq)
			})
		}
	}
}

func TestDisallowAnonymousWithdraw(t *testing.T) {
	ctx := context.Background()
	env, err := testenv.NewTestEnvironment(ctx)
	if err != nil {
		t.Fatal(err)
	}
	err = env.Start(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer env.Stop(ctx)
	env.WaitForLoad()

	depositBig := bitcoin.Must(bitcoin.BTCAmountFromStringedFloat("1"))

	processingSettings := testenv.DefaultSettings
	processingSettings.CallbackURL = env.CallbackURL
	processingSettings.AllowWithdrawalWithoutID = false

	err = env.StartProcessing(ctx, &processingSettings)

	if err != nil {
		t.Fatal(err)
	}
	defer env.StopProcessing(ctx)
	env.WaitForProcessing()
	_, err = env.NewWebsocketListener(0)
	if err != nil {
		t.Fatal(err)
	}

	// deposit some money so there is what to withdraw
	ourBalance := getStableBalanceOrFail(t, env)
	clientAccount := testGenerateClientWalletWithMetainfo(t, env, initialTestMetainfo, -1)
	tx := testMakeDeposit(t, env, clientAccount.Address, depositBig, initialTestMetainfo)
	checkDepositCompletedNormally(t, env, tx, ourBalance, depositBig)

	clientBalance := getStableClientBalanceOrFail(t, env)
	withdrawAddress := getNewAddressForWithdrawOrFail(t, env)

	req := &wallet.WithdrawRequest{
		Address: withdrawAddress,
		Amount:  withdrawAmountSmall,
		Fee:     withdrawFee,
	}

	_, err = env.ProcessingClient.Withdraw(req)

	if err == nil {
		t.Fatal("Expected that withdraw without id would cause an error")
	}

	req.ID = uuid.Must(uuid.NewV4())

	resp, err := env.ProcessingClient.Withdraw(req)

	if err != nil {
		t.Fatal(err)
	}
	checkWithdrawCompletedNormally(t, env, req, resp, clientBalance)
}

func runDepositReliabilityCase(t *testing.T, env *testenv.TestEnvironment, ctx context.Context, failure failureType, moment failureMoment, eventSeq int) int {
	qtForConn := make(map[*pgmitm.Connection]queryType)
	failureCount := 1

	if failure == FailureConnFailureMultiple {
		failureCount = 2
	}

	processingSettings := testenv.DefaultSettings
	processingSettings.CallbackURL = env.CallbackURL

	allowMITMUpstreamConnFailure := failure == FailureDBCrash

	mitm := addPgMITMOrFail(env, &processingSettings, t, allowMITMUpstreamConnFailure)

	defer mitm.Shutdown()

	err := env.StartProcessing(ctx, &processingSettings)

	if err != nil {
		t.Fatal(err)
	}
	defer env.StopProcessing(ctx)
	env.WaitForProcessing()
	_, err = env.NewWebsocketListener(eventSeq + 1)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if len(env.WebsocketListeners) > 0 {
			env.WebsocketListeners[0].Stop()
			env.WebsocketListeners = env.WebsocketListeners[1:]
		}
	}()

	clientAccount := testGenerateClientWalletWithMetainfo(t, env, initialTestMetainfo, -1)

	mitm.AddClientMsgHandler(func(msg pgproto3.FrontendMessage, c *pgmitm.Connection) {
		if parseMsg, ok := msg.(*pgproto3.Parse); ok {
			qtForConn[c] = determineQueryTypeFromParseMsg(parseMsg)
		}
	})

	failureDone := make(chan struct{})

	mitm.AddClientMsgHandler(func(msg pgproto3.FrontendMessage, c *pgmitm.Connection) {
		switch m := msg.(type) {
		case *pgproto3.Query:
			if moment == FailureMomentCommit && m.String == "COMMIT" {
				causeFailure(t, env, c, mitm, failure, &failureCount, ctx)
				if failureCount == 0 {
					close(failureDone)
					failureCount--
				}
			}
		case *pgproto3.Bind:
			qt := qtForConn[c]
			switch {
			case moment == FailureMomentTxWrite && qt == queryTxWrite:
				fallthrough
			case moment == FailureMomentEventWrite && qt == queryEventWrite:
				fallthrough
			case moment == FailureMomentLastSeenBlockHashWrite && qt == queryMetainfoWrite && string(m.Parameters[0]) == "last_seen_block_hash":
				fallthrough
			case moment == FailureMomentHTTPSentSeqWrite && qt == queryMetainfoWrite && string(m.Parameters[0]) == "last_http_sent_seq":
				causeFailure(t, env, c, mitm, failure, &failureCount, ctx)
				if failureCount == 0 {
					close(failureDone)
					failureCount--
				}
			}
		}
	})

	ourBalance := getStableBalanceOrFail(t, env)

	tx := testMakeDeposit(t, env, clientAccount.Address, testDepositAmount, initialTestMetainfo)

	<-failureDone

	recoverable := restoreAfterFailure(t, env, mitm, ctx, failure, moment, false)

	if !recoverable {
		return 0
	}

	checkDepositCompletedNormally(t, env, tx, ourBalance, testDepositAmount)

	return env.WebsocketListeners[0].LastSeq
}

func runWithdrawReliabilityCase(t *testing.T, env *testenv.TestEnvironment, ctx context.Context, failure failureType, moment failureMoment, eventSeq int) int {
	depositBig := bitcoin.Must(bitcoin.BTCAmountFromStringedFloat("5"))

	qtForConn := make(map[*pgmitm.Connection]queryType)
	failureCount := 1

	if failure == FailureConnFailureMultiple {
		failureCount = 2
	}

	processingSettings := testenv.DefaultSettings
	processingSettings.CallbackURL = env.CallbackURL

	allowMITMUpstreamConnFailure := failure == FailureDBCrash

	mitm := addPgMITMOrFail(env, &processingSettings, t, allowMITMUpstreamConnFailure)

	defer mitm.Shutdown()

	err := env.StartProcessing(ctx, &processingSettings)

	if err != nil {
		t.Fatal(err)
	}
	defer env.StopProcessing(ctx)
	env.WaitForProcessing()
	_, err = env.NewWebsocketListener(eventSeq + 1)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if len(env.WebsocketListeners) > 0 {
			env.WebsocketListeners[0].Stop()
			env.WebsocketListeners = env.WebsocketListeners[1:]
		}
	}()

	// deposit some money so there is what to withdraw
	ourBalance := getStableBalanceOrFail(t, env)
	clientAccount := testGenerateClientWalletWithMetainfo(t, env, initialTestMetainfo, -1)
	tx := testMakeDeposit(t, env, clientAccount.Address, depositBig, initialTestMetainfo)
	checkDepositCompletedNormally(t, env, tx, ourBalance, depositBig)

	mitm.AddClientMsgHandler(func(msg pgproto3.FrontendMessage, c *pgmitm.Connection) {
		if parseMsg, ok := msg.(*pgproto3.Parse); ok {
			qtForConn[c] = determineQueryTypeFromParseMsg(parseMsg)
		}
	})

	failureDone := make(chan struct{})

	mitm.AddClientMsgHandler(func(msg pgproto3.FrontendMessage, c *pgmitm.Connection) {
		switch m := msg.(type) {
		case *pgproto3.Query:
			if moment == FailureMomentCommit && m.String == "COMMIT" {
				causeFailure(t, env, c, mitm, failure, &failureCount, ctx)
				if failureCount == 0 {
					close(failureDone)
					failureCount--
				}
			}
		case *pgproto3.Bind:
			qt := qtForConn[c]
			switch {
			case moment == FailureMomentTxWrite && qt == queryTxWrite:
				fallthrough
			case moment == FailureMomentEventWrite && qt == queryEventWrite:
				fallthrough
			case moment == FailureMomentLastSeenBlockHashWrite && qt == queryMetainfoWrite && string(m.Parameters[0]) == "last_seen_block_hash":
				fallthrough
			case moment == FailureMomentHTTPSentSeqWrite && qt == queryMetainfoWrite && string(m.Parameters[0]) == "last_http_sent_seq":
				causeFailure(t, env, c, mitm, failure, &failureCount, ctx)
				if failureCount == 0 {
					close(failureDone)
					failureCount--
				}
			}
		}
	})

	ourBalance = getStableBalanceOrFail(t, env)
	clientBalance := getStableClientBalanceOrFail(t, env)
	withdrawAddress := getNewAddressForWithdrawOrFail(t, env)

	req := &wallet.WithdrawRequest{
		Address: withdrawAddress,
		Amount:  withdrawAmountSmall,
		Fee:     withdrawFee,
	}

	var resp *wallet.WithdrawRequest
	withdrawDone := make(chan struct{})

	go func() {
		defer close(withdrawDone)
		resp, err = env.ProcessingClient.Withdraw(req)
	}()

	<-failureDone

	recoverable := restoreAfterFailure(t, env, mitm, ctx, failure, moment, true)

	<-withdrawDone

	switch {
	case failure == FailureProcessingCrash && moment != FailureMomentHTTPSentSeqWrite && err == nil:
		t.Fatal("Expected Withdraw to return error when processing crashed")
	case failure != FailureProcessingCrash && err != nil:
		t.Fatal(err)
	}

	if !recoverable {
		return 0
	}

	checkWithdrawCompletedNormally(t, env, req, resp, clientBalance)

	return env.WebsocketListeners[0].LastSeq
}

func checkDepositCompletedNormally(t *testing.T, env *testenv.TestEnvironment, tx *txTestData, ourBalance, amount bitcoin.BTCAmount) {
	notification := env.GetNextCallbackNotificationWithTimeout(t)
	tx.id = notification.ID
	checkNotificationFieldsForNewDeposit(t, notification, tx)
	event := env.WebsocketListeners[0].GetNextMessageWithTimeout(t)

	data := event.Data.(*wallet.TxNotification)
	if got, want := event.Type, events.NewIncomingTxEvent; got != want {
		t.Errorf("Unexpected event type for new deposit, wanted %s, got %s:",
			want, got)
	}
	if data.ID != tx.id {
		t.Errorf("Expected that tx id in websocket and http callback "+
			"notification will be the same, but they are %s %s",
			tx.id, data.ID)
	}
	checkNotificationFieldsForNewDeposit(t, data, tx)

	checkBalance(t, env, ourBalance, ourBalance+amount)

	tx.mineOrFail(t, env)

	testDepositFullyConfirmed(t, env, tx)

	checkBalance(t, env, ourBalance+amount, ourBalance+amount)
}

func checkWithdrawCompletedNormally(t *testing.T, env *testenv.TestEnvironment, req, resp *wallet.WithdrawRequest, clientBalance bitcoin.BTCAmount) {
	checkClientWithdrawRequest(t, resp, req)

	tx := &txTestData{
		id:      resp.ID,
		address: req.Address,
		amount:  withdrawAmountSmall,
		fee:     withdrawFee,
	}

	expectedClientBalanceAfterWithdraw := clientBalance + withdrawAmountSmall - withdrawFee

	testWithdrawNewTransaction(t, env, tx, clientBalance, expectedClientBalanceAfterWithdraw)
	tx.mineOrFail(t, env)
	testWithdrawFullyConfirmed(t, env, tx, expectedClientBalanceAfterWithdraw)
}

func determineQueryTypeFromParseMsg(parseMsg *pgproto3.Parse) queryType {
	switch {
	case strings.HasPrefix(parseMsg.Query, "INSERT INTO transactions"):
		return queryTxWrite
	case strings.HasPrefix(parseMsg.Query, "INSERT INTO events"):
		return queryEventWrite
	case strings.HasPrefix(parseMsg.Query, "INSERT INTO metadata"):
		return queryMetainfoWrite
	default:
		return queryUndefined
	}

}

func causeFailure(t *testing.T, env *testenv.TestEnvironment, c *pgmitm.Connection, mitm *pgmitm.PgMITM, failure failureType, count *int, ctx context.Context) {
	if *count <= 0 {
		return
	}
	*count--
	switch failure {
	case FailureConnFailureMultiple:
		fallthrough
	case FailureConnFailure:
		c.Shutdown()
	case FailureProcessingCrash:
		log.Printf("Killing processing")
		err := env.KillProcessing(ctx)
		if err != nil {
			t.Fatal(err)
		}
	case FailureDBCrash:
		err := env.KillDatabase(ctx, false)
		if err != nil {
			t.Fatal(err)
		}
	default:
		panic(fmt.Sprintf("Unexpected failure type %v", failure))
	}
}

func restoreAfterFailure(t *testing.T, env *testenv.TestEnvironment, mitm *pgmitm.PgMITM, ctx context.Context, failure failureType, moment failureMoment, isWithdraw bool) bool {
	switch failure {
	case FailureProcessingCrash:
		if env.Processing != nil {
			t.Fatal("Expected processing to be down after crash")
		}
		lastSeq := env.WebsocketListeners[0].LastSeq
		env.WebsocketListeners = env.WebsocketListeners[1:]
		err := env.StartProcessing(ctx, env.ProcessingSettings)
		if err != nil {
			t.Fatal(err)
		}

		// for withdraw, crash at any moment between sending money and writing
		// info about it to DB is unrecoverable
		recoverable := !isWithdraw && moment != FailureMomentHTTPSentSeqWrite

		if !recoverable {
			handleUnrecoverableCrash(t, env, ctx)
			return false
		}

		env.WaitForProcessing()
		_, err = env.NewWebsocketListener(lastSeq + 1)
		if err != nil {
			t.Fatal(err)
		}
		return true
	case FailureDBCrash:
		err := env.LaunchDatabaseContainer(ctx)
		if err != nil {
			t.Fatal(err)
		}
		env.WaitForDatabase()
	}
	return true
}

func handleUnrecoverableCrash(t *testing.T, env *testenv.TestEnvironment, ctx context.Context) {
	err := util.WaitForEvent(func() error {
		state, err := env.GetProcessingContainerState(ctx)
		if err != nil {
			return err
		}
		switch state.Status {
		case "exited":
		case "dead":
		default:
			return fmt.Errorf(
				"Expected processing container to exit, but it's state "+
					"is %s", state.Status,
			)
		}
		if state.ExitCode == 0 {
			return fmt.Errorf(
				"Expected processing container to exit with nonzero code",
			)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	//cleanup
	env.KillProcessing(ctx)
	err = env.Stop(ctx)
	if err != nil {
		t.Fatal(err)
	}
	err = env.Start(ctx)
	if err != nil {
		t.Fatal(err)
	}
	env.WaitForLoad()
}
