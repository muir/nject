package nject

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type (
	b1  bool
	b2  bool
	b3  bool
	b4  bool
	b5  bool
	b6  bool
	b7  bool
	b8  bool
	b9  bool
	b10 bool
	b11 bool
	b12 bool
)

func TestHealthRegression(t *testing.T) {
	wrapTest(t, func(t *testing.T) {
		var i1v i1
		var i2v i2
		seq := Sequence("Bind",
			Sequence("Service",
				Desired(Provide("Global", func() i1 { return i1v })),
				Desired(Provide("ServiceLog", func() i2 { return i2v })),
				Sequence("Common",
					Desired(Provide("LogBegin", func(i2, s1) i3 { return nil })),
					Desired(Provide("Writer", func(i3, i4) i4prime { return nil })),
					Desired(Provide("WriteJSON", func(func() ie, i4prime, i3) {})),
					Desired(Provide("IfError", func(func() (ie, a1), i3, i4prime) ie { return nil })),
					Desired(Provide("AsErrors", func(func() error, i3, i4prime) a1 { return nil })),
					Desired(Provide("SaveRequest", func(i4, s1) (TerminalError, []byte) { return nil, nil })),
				),
			),
			Provide("Handler", func(i4, s1) {}))
		var invoke func(i4, s1)
		assert.NoError(t, seq.Bind(&invoke, nil))
	})
}

func TestApiAggRegression(t *testing.T) {
	wrapTest(t, func(t *testing.T) {
		var i2v i2
		var i2vimp i2imp
		i2v = i2vimp
		seq := Sequence("Bind",
			Sequence("Service",
				Desired(Provide("RedPub", func() s2 { return "" })),
				func() i2 { return i2v }, // service log
				Provide("Cors", func(i4, s1) {}),
				Sequence("Common",
					Desired(Provide("LogBegin", func(i2, s1) i3 { return nil })),
					Desired(Provide("Writer", func(i3, i4) i4prime { return nil })),
					Desired(Provide("WriteJSON", func(func() ie, i4prime, i3) {})),
					Desired(Provide("IfError", func(func() (ie, a1), i3, i4prime) ie { return nil })),
					Desired(Provide("AsErrors", func(func() error, i3, i4prime) a1 { return nil })),
					Desired(Provide("SaveRequest", func(i4, s1) (TerminalError, s3) { return nil, "" })),
				),
			),
			Provide("Handler", func(i3, s1, s2, s3) (ie, error) { return nil, nil }))
		var invoke func(i4, s1)
		assert.NoError(t, seq.Bind(&invoke, nil))
	})
}

func TestOverEagerTerminalError(t *testing.T) {
	wrapTest(t, func(t *testing.T) {
		assert.NotPanics(t, func() {
			assert.NoError(t, Run("over-eager",
				func() (TerminalError, s1) {
					t.Log("terminal error func is running")
					return fmt.Errorf("i should not run since s1 is not used"), ""
				},
				func() error {
					t.Log("final func is running")
					return nil
				}))
		})
	})
}

func TestErroringTerminalError(t *testing.T) {
	wrapTest(t, func(t *testing.T) {
		assert.NotPanics(t, func() {
			err := Run("gen-error",
				func() (TerminalError, s1) {
					return fmt.Errorf("some error"), ""
				},
				func(s1) error {
					return fmt.Errorf("some other error")
				})
			assert.Error(t, err)
			assert.Equal(t, "some error", err.Error())
		})
	})
}

func TestStatusRegression(t *testing.T) {
	wrapTest(t, func(t *testing.T) {
		var i1v i1
		var i2v i2
		seq := Sequence("Bind",
			Sequence("Service",
				Desired(Provide("Global", func() i1 { return i1v })),
				Desired(Provide("ServiceLog", func() i2 { return i2v })),
				Sequence("Common",
					Desired(Provide("LogBegin", func(i2, s1) i3 { return nil })),
					Desired(Provide("Writer", func(i3, i4) i4prime { return nil })),
					Desired(Provide("WriteJSON", func(func() ie, i4prime, i3) {})),
					Desired(Provide("IfError", func(func() (ie, a1), i3, i4prime) ie { return nil })),
					Desired(Provide("AsErrors", func(func() error, i3, i4prime) a1 { return nil })),
					Desired(Provide("SaveRequest", func(i4, s1) (TerminalError, []byte) { return nil, nil })),
				),
			),
			Provide("Handler", func(i4prime) error { return nil }))
		var invoke func(i4, s1)
		// Handler returns error.  The only consumer of error is AsErrors.
		// AsErrors requires i3 (provided by LogBegin), i4prime (provided by Writer) and returns a1.
		// a1 is onsumed by IfError which also consumes ie.  There is no source of ie.
		// Thus an error.
		assert.Error(t, seq.Bind(&invoke, nil))
	})
}

func TestOversizeRequestRegression1(t *testing.T) {
	wrapTest(t, func(t *testing.T) {
		wjCalled := false
		ieCalled := false
		aeCalled := false
		odCalled := false
		txCalled := false
		seq := Sequence("Bind",
			Sequence("Service",
				Sequence("Common",
					Provide("LogProvider", func() i2 { return nil }),
					Provide("LogBegin", func(i2, s1) i3 { return nil }),
					Provide("Writer", func(i3, i4) i4prime { return nil }),
					Provide("WriteJSON", func(i func() ie, x i4prime, y i3) { wjCalled = true; i() }),
					Desired(Provide("IfError", func(i func() (ie, a1), x i3, y i4prime) ie { ieCalled = true; i(); return nil })),
					Provide("AsErrors", func(i func() error, x i3, y i4prime) a1 { aeCalled = true; i(); return nil }),
					Provide("SaveRequest", func(i4, s1) (TerminalError, s5) { return nil, "" }),
					Sequence("OpenDB",
						Provide("DBOpen", NotCacheable(func(s1) (s7, TerminalError) {
							odCalled = true
							require.Fail(t, "Failing because DBOpen should not be called")
							return "", nil
						})),
						MustConsume(Provide("DBClose", func(i func(s7), x s7) { i(x) })),
					),
					// Tx should be excluded from the chain because AsError is the only
					// required match for error
					MustConsume(Provide("Tx", func(i func(s8, s9) error, w s1, x s7, y i3, z i4prime) error { txCalled = true; i("", ""); return nil })),
					Provide("ParentTx", func(s8) s8prime { return "" }),
				),
			),
			Provide("Handler", func(i3, s5) (ie, error) { return nil, nil }))
		var invoke func(i4, s1)
		require.NoError(t, seq.Bind(&invoke, nil))
		invoke(nil, "")
		assert.True(t, wjCalled, "write json")
		assert.True(t, ieCalled, "if error")
		assert.True(t, aeCalled, "as error")
		assert.False(t, odCalled, "open db")
		assert.False(t, txCalled, "tx")
	})
}

func TestMissingAutoDesiredRegression(t *testing.T) {
	wrapTest(t, func(t *testing.T) {
		oneCalled := false
		twoCalled := false
		var initFunc func()
		var invoke http.HandlerFunc
		require.NoError(t, Sequence("test",
			Cacheable(func(*Debugging) string {
				oneCalled = true
				return ""
			}),
			func(string, *http.Request, *Debugging) {
				twoCalled = true
			},
			Cacheable(func(*Debugging) int {
				return 0
			}),
			func(http.ResponseWriter, int, *Debugging) {},
		).Bind(&invoke, &initFunc))
		initFunc()
		assert.True(t, oneCalled)
		assert.False(t, twoCalled)
		invoke(nil, nil)
		assert.True(t, twoCalled)
	})
}

func TestLotsOfUnusedRegresssion(t *testing.T) {
	wrapTest(t, func(t *testing.T) {
		var initer func()
		var invoker http.HandlerFunc
		assert.NoError(t,
			Sequence("regression",
				Provide("ServiceLog", func() s1 { return "" }),
				Provide("LogBegin", func(func(s2), *http.Request) s1 { return "" }),
				Provide("Writer", func(http.ResponseWriter, s2) s3 { return "" }),
				Provide("WriteJSON", func(func() ie, s3, s2) {}),
				Provide("IfError", func(func() (ie, i1), s2, s3) ie { return nil }),
				Provide("AsErrors", func(func() error, s2, s3) i1 { return nil }),
				Provide("BC1", func(http.ResponseWriter, *http.Request) (TerminalError, s4) { return nil, "" }),
				Provide("HM0", func(s2) {}),
				Provide("HM1", func(http.ResponseWriter, *http.Request) {}),
			).Bind(&invoker, &initer))
	})
}

func TestRepublishRegression(t *testing.T) {
	wrapTest(t, func(t *testing.T) {
		assert.NoError(t, Run("test",
			Provide("SupplyError", Required(func() error {
				return fmt.Errorf("this error should be buried by the next func")
			})),
			// This function must be included to avoid an error return from the final func
			Provide("ResetError", func(error) error {
				return nil
			}),
			Provide("ReturnReceivedError", func(err error) error {
				return err
			}),
		))
	})
}

func TestMissingPaymentProviderRegression(t *testing.T) {
	wrapTest(t, func(t *testing.T) {
		var initer func()
		var invoker http.HandlerFunc
		cctCalled := false
		mcCalled := false
		eCalled := true
		require.NoError(t,
			Sequence("regression",
				Provide("IsProduction", func() b1 { return false }),
				Provide("BaseURL", func() s9 { return "" }),
				Provide("PaymentProvider", func() s8 { return "" }),
				Provide("AbortIfPurchased", func() i9 { return nil }),
				Provide("OverrideMaxPrice", func() i8 { return nil }),
				Provide("QuoteServer", func() i7 { return nil }),
				Provide("DocSigningKey", func() i6 { return nil }),
				Provide("QuoteRater", func() i5 { return nil }),
				Provide("QuoteChecker", func() i4 { return nil }),
				Provide("QuoteUpdater", func() i3 { return nil }),
				Provide("ServiceLog", func() s1 { return "" }),
				Provide("CORS", func(http.ResponseWriter, *http.Request) {}),
				Provide("LogBegin", func(i func(s2), a *http.Request, b s1) {
					i("")
				}),
				Provide("LogInjectors", func(s2, *Debugging) {}),
				Provide("Writer", func(http.ResponseWriter, s2) s3 { return "" }),
				Provide("WriteJSON", func(i func() ie, a s3, b s2) {
					i()
				}),
				Provide("IfError", func(i func() (ie, i1), a s2, b s3) ie {
					i()
					return nil
				}),
				Provide("AsErrors", func(i func() error, a s2, b s3) i1 {
					i()
					return nil
				}),
				Provide("SaveRequest", func(http.ResponseWriter, *http.Request) (TerminalError, s4) { return nil, "" }),
				Provide("ModifyContext", func(*http.Request) *http.Request {
					mcCalled = true
					return nil
				}),
				Provide("DBOpen", func(*http.Request) (s5, TerminalError) {
					return "", nil
				}),
				Provide("DBClose", func(i func(s5), a s5) {
					i("")
				}),
				Provide("TimeTravelHeader", func() i2 { return nil }),
				Provide("ParseClientQuote", func(s2, s4) (TerminalError, s7) { return nil, "" }),
				Provide("ConvertCardToken", func(s2, s7, s8) (TerminalError, s7) {
					cctCalled = true
					return nil, ""
				}),
				Provide("WrapTx", func(i func(b2, b3) error, a *http.Request, b s5, c s2, d s3) error {
					i(true, true)
					return nil
				}),
				Provide("ParentTx", func(b2) b4 { return false }),
				Provide("GetSessionId", func(s2, *http.Request) (TerminalError, b5) { return nil, false }),
				Provide("LoadSavedQuote", func(s2, b4, b5) (TerminalError, b6) { return nil, true }),
				Provide("VariationsRuleChecker", func(s2, i8, i6, s9, b5, b4) b7 { return false }),
				Provide("ReassembleQuote", func(s2, s7, b6, i9) (TerminalError, b8, b9) { return nil, false, true }),
				Provide("CheckAndUpdateQuote", func(*http.Request, s2, s5, b7, i3, i4, i5, b8, b5, b2, b9) (TerminalError, b10, b11) {
					return nil, false, true
				}),
				Provide("TransformResponse", func(i func() b12) ie {
					i()
					return nil
				}),
				Provide("endpoint", func(s2, s5, b4, *http.Request, b5, b10, b11) (b12, error) {
					eCalled = true
					return true, nil
				}),
			).Bind(&invoker, &initer))
		initer()
		invoker(nil, nil)
		assert.True(t, mcCalled, "modify context")
		assert.True(t, cctCalled, "convert card token")
		assert.True(t, eCalled, "endpoint")
	})
}

func TestExtraneousConversionRegression(t *testing.T) {
	wrapTest(t, func(t *testing.T) {
		var initer func()
		var invoker http.HandlerFunc
		cctCalled := false
		mcCalled := false
		eCalled := true
		require.NoError(t,
			Sequence("regression",
				Provide("IsProduction", func() b1 { return false }),
				Provide("BaseURL", func() s9 { return "" }),
				Provide("PaymentProvider", func() s8 { return "" }),
				Provide("AbortIfPurchased", func() i9 { return nil }),
				Provide("OverrideMaxPrice", func() i8 { return nil }),
				Provide("QuoteServer", func() i7 { return nil }),
				Provide("DocSigningKey", func() i6 { return nil }),
				Provide("QuoteRater", func() i5 { return nil }),
				Provide("QuoteChecker", func() i4 { return nil }),
				Provide("QuoteUpdater", func() i3 { return nil }),
				Provide("ServiceLog", func() s1 { return "" }),
				Provide("CORS", func(http.ResponseWriter, *http.Request) {}),
				Provide("LogBegin", func(i func(s2), a *http.Request, b s1) {
					i("")
				}),
				Provide("LogInjectors", func(s2, *Debugging) {}),
				Provide("Writer", func(http.ResponseWriter, s2) s3 { return "" }),
				Provide("WriteJSON", func(i func() ie, a s3, b s2) {
					i()
				}),
				Provide("IfError", func(i func() (ie, i1), a s2, b s3) ie {
					i()
					return nil
				}),
				Provide("AsErrors", func(i func() error, a s2, b s3) i1 {
					i()
					return nil
				}),
				Provide("SaveRequest", func(http.ResponseWriter, *http.Request) (TerminalError, s4) { return nil, "" }),
				Provide("ModifyContext", func(*http.Request) *http.Request {
					mcCalled = true
					return nil
				}),
				Provide("DBOpen", func(*http.Request) (s5, TerminalError) {
					return "", nil
				}),
				Provide("DBClose", func(i func(s5), a s5) {
					i("")
				}),
				Provide("TimeTravelHeader", func() i2 { return nil }),
				Provide("ParseClientQuote", func(s2, s4) (TerminalError, s7) { return nil, "" }),
				Provide("ConvertCardToken", func(s2, s7, s8) (TerminalError, s7) {
					cctCalled = true
					return nil, ""
				}),
				Provide("WrapTx", func(i func(b2, b3) error, a *http.Request, b s5, c s2, d s3) error {
					i(true, true)
					return nil
				}),
				Provide("ParentTx", func(b2) b4 { return false }),
				Provide("GetSessionId", func(s2, *http.Request) (TerminalError, b5) { return nil, false }),
				Provide("LoadSavedQuote", func(s2, b4, b5) (TerminalError, b6) { return nil, true }),
				Provide("VariationsRuleChecker", func(s2, i8, i6, s9, b5, b4) b7 { return false }),
				Provide("ReassembleQuote", func(s2, s7, b6, i9) (TerminalError, b8, b9) { return nil, false, true }),
				Provide("CheckAndUpdateQuote", func(*http.Request, s2, s5, b7, i3, i4, i5, b8, b5, b2, b9) (TerminalError, b10, b11) {
					return nil, false, true
				}),
				Provide("TransformResponse", func(i func() b12) ie {
					i()
					return nil
				}),
				Provide("endpoint", func(s2, *http.Request, s3, s4, b4, s5, b1) (b12, error) {
					eCalled = true
					return true, nil
				}),
			).Bind(&invoker, &initer))
		initer()
		invoker(nil, nil)
		assert.True(t, mcCalled, "modify context")
		assert.False(t, cctCalled, "convert card token")
		assert.True(t, eCalled, "endpoint")
	})
}

// http.ResponseWriter
type i016 interface {
	x016()
}

// *http.Request
type s017 int

// server.isProduction
type s050 int

// quotes.BaseURL
type s036 int

// payment.Provider
type i035 interface {
	x035()
}

// quotes.AbortIfPurchased
type s051 int

// rules.OverrideMaximumAnnualPrice
type s052 int

// *quotes.Quotes
type s053 int

// quotes.DocumentSigningKey
type s038 int

// quotes.QuoteRater
type s054 int

// quotes.Checker
type s055 int

// quotes.EisQuoteUpdater
type s056 int

// logger.ServiceLog
type i018 interface {
	x018()
}

// logger.RequestLog
type i019 interface {
	x019()
}

// wrap.EnhancedWriter
type i020 interface {
	x020()
}
type i021 interface{} // wrap.JSONResult
// httptools.Errors
type s022 int

// wrap.RequestBody
type s023 int

// sqlutils.DBH
type i029 interface {
	x029()
}

// quotes.unusedType
type s041 int

// quotes.ClientQuote
type s057 int

// backoff.Tx
type i030 interface {
	x030()
}

// wrap.TxDone
type i031 interface {
	x031()
}

// sqlutils.Tx
type i032 interface {
	x032()
}

// quotes.SessionId
type s043 int

// *quotes.AdvisoryLockedQueryable
type s044 int

// quotes.LoadedQuote
type s058 int

// quotes.VariationRulesChecker
type s059 int

// quotes.MergedQuote
type s060 int

// quotes.BeforeMergeRatesInvalid
type s061 int

// *models.QuoteV1
type s046 int

// models.Errors
type s045 int

// *server.QuotesResponse
type s048 int

func TestRegressionPrior(t *testing.T) {
	wrapTest(t, func(t *testing.T) {
		called := make(map[string]int)
		var invoker func(i016, s017)
		var initer func()
		require.NoError(t,
			Sequence("regression",
				Cacheable(Provide("IsProduction", func() s050 { called["IsProduction"]++; return 0 })),         // included
				Cacheable(Provide("BaseURL", func() s036 { called["BaseURL"]++; return 0 })),                   // included
				Cacheable(Provide("PaymentProvider", func() i035 { called["PaymentProvider"]++; return nil })), // included
				Cacheable(Provide("AbortIfPurchased", func() s051 { called["AbortIfPurchased"]++; return 0 })), // included
				Cacheable(Provide("OverrideMaxPrice", func() s052 { called["OverrideMaxPrice"]++; return 0 })), // included
				Provide("QuoteServer", s053(0)),
				Provide("DocSigningKey", s038(0)),
				Cacheable(Provide("QuoteRater", func() s054 { called["QuoteRater"]++; return 0 })),     // included
				Cacheable(Provide("QuoteChecker", func() s055 { called["QuoteChecker"]++; return 0 })), // included
				Cacheable(Provide("QuoteUpdater", func() s056 { called["QuoteUpdater"]++; return 0 })), // included
				Cacheable(Provide("ServiceLog", func() i018 { called["ServiceLog"]++; return nil })),   // included
				Provide("CORS", func(_ i016, _ s017) { called["CORS"]++ }),                             // included
				Provide("LogBegin", func(inner func(i019), _ s017, _ i018) {
					called["LogBegin"]++
					inner(nil)
				}), // included
				Desired(Provide("InjectorChainDebugging", func(_ i019, _ *Debugging) { called["InjectorChainDebugging"]++ })), // included
				Provide("base-collection-2", func(_ i016, _ i019) i020 { called["base-collection-2"]++; return nil }),         // included
				Provide("WriteJSON", func(inner func() i021, _ i020, _ i019) {
					called["WriteJSON"]++
					inner()
				}), // included
				Desired(Provide("IfError", func(inner func() (i021, s022), _ i019, _ i020) i021 {
					called["IfError"]++
					inner()
					return nil
				})), // included
				Provide("AsErrors", func(inner func() error, _ i019, _ i020) s022 {
					called["AsErrors"]++
					inner()
					return 0
				}), // included
				Provide("SaveRequest", func(_ i016, _ s017) (TerminalError, s023) { called["SaveRequest"]++; return nil, 0 }),            // included
				Provide("ModifyContext", func(_ s017) s017 { called["ModifyContext"]++; return 0 }),                                      // included
				MustConsume(NotCacheable(Provide("DBOpen", func(_ s017) (i029, TerminalError) { called["DBOpen"]++; return nil, nil }))), // included
				MustConsume(Provide("DBClose", func(inner func(i029), _ i029) {
					called["DBClose"]++
					inner(nil)
				})), // included
				Provide("TimeTravelHeader", func() s041 { called["TimeTravelHeader"]++; return 0 }),
				Provide("ParseClientQuote", func(_ i019, _ s023) (TerminalError, s057) { called["ParseClientQuote"]++; return nil, 0 }),                        // included
				MustConsume(Provide("ConverteCardToken", func(_ i019, _ s057, _ i035) (s057, TerminalError) { called["ConverteCardToken"]++; return 0, nil })), // included
				MustConsume(Provide("Tx", func(inner func(i030, i031) error, _ s017, _ i029, _ i019, _ i020) error {
					called["Tx"]++
					inner(nil, nil)
					return nil
				})),
				Provide("ConsumeTxDone", func(_ i031) { called["ConsumeTxDone"]++ }),
				MustConsume(Provide("ParentTx", func(_ i030) i032 { called["ParentTx"]++; return nil })),
				Provide("GetSessionID", func(_ i019, _ s017) (TerminalError, s043) { called["GetSessionID"]++; return nil, 0 }), // included
				Provide("AdvisoryLockQuote", func(inner func(s044) error, _ i019, _ i029, _ s043) error {
					called["AdvisoryLockQuote"]++
					inner(0)
					return nil
				}), // included
				Provide("LoadSavedQuote", func(_ i019, _ s044, _ s043) (TerminalError, s058) { called["LoadSavedQuote"]++; return nil, 0 }), // included
				Provide("VariationsRuleChecker", func(_ i019, _ s052, _ s038, _ s036, _ s043, _ s044) s059 {
					called["VariationsRuleChecker"]++
					return 0
				}), // included
				Provide("ReassembleQuote", func(_ i019, _ s057, _ s058, _ s051) (TerminalError, s060, s061) {
					called["ReassembleQuote"]++
					return nil, 0, 0
				}), // included
				Provide("CheckAndUpdateQuote", func(_ s017, _ i019, _ i029, _ s059, _ s056, _ s055, _ s054, _ s060, _ s043, _ s044, _ s061) (TerminalError, s046, s045) {
					called["CheckAndUpdateQuote"]++
					return nil, 0, 0
				}),
				Provide("endpoint-0", func(inner func() s048) i021 {
					called["endpoint-0"]++
					inner()
					return nil
				}), // included
				Required(Provide("endpoint-1", func(_ i019, _ s017, _ i020, _ s023, _ i029, _ s050) (s048, error) {
					called["endpoint-1"]++
					return 0, nil
				})), // included
			).Bind(&invoker, &initer))
		initer()
		invoker(nil, 0)
		assert.Equal(t, 0, called["ParseClientQuote"])
	})
}

// *http.Request
type s032 int

// server.isProduction
type s062 int

// *quotes.Quotes
type s063 int

// quotes.Checker
type s064 int

// quotes.EisQuoteUpdater
type s065 int

// logger.ServiceLog
type i033 interface {
	x033()
}

// logger.RequestLog
type i034 interface {
	x034()
}
type i036 interface{} // wrap.JSONResult
// httptools.Errors
type s037 int

// wrap.RequestBody
type i044 interface {
	x044()
}

// quotes.ClientQuote
type s066 int

// backoff.Tx
type i045 interface {
	x045()
}

// wrap.TxDone
type i046 interface {
	x046()
}

// sqlutils.Tx
type i047 interface {
	x047()
}

// quotes.LoadedQuote
type s067 int

// server.PurchaseFailureDummyType
type s068 int

// quotes.VariationRulesChecker
type s069 int

// quotes.MergedQuote
type s070 int

// quotes.BeforeMergeRatesInvalid
type s071 int

func TestRegression7642(t *testing.T) {
	wrapTest(t, func(t *testing.T) {
		called := make(map[string]int)
		var invoker func(i031, s032)
		var initer func()
		require.NoError(t,
			Sequence("regression",
				Cacheable(Provide("IsProduction", func() s060 { called["IsProduction"]++; return 0 })),         // included
				Cacheable(Provide("AbortIfPurchased", func() s061 { called["AbortIfPurchased"]++; return 0 })), // included
				Cacheable(Provide("OverrideMaxPrice", func() s062 { called["OverrideMaxPrice"]++; return 0 })), // included
				Provide("QuoteServer", s023(0)),
				Cacheable(Provide("QuoteRater", func() s063 { called["QuoteRater"]++; return 0 })),     // included
				Cacheable(Provide("QuoteChecker", func() s064 { called["QuoteChecker"]++; return 0 })), // included
				Cacheable(Provide("QuoteUpdater", func() s065 { called["QuoteUpdater"]++; return 0 })), // included
				Cacheable(Provide("ServiceLog", func() i033 { called["ServiceLog"]++; return nil })),   // included
				Provide("CORS", func(_ i031, _ s032) { called["CORS"]++ }),                             // included
				Provide("LogBegin", func(inner func(i034), _ s032, _ i033) {
					called["LogBegin"]++
					inner(nil)
				}), // included
				Desired(Provide("InjectorChainDebugging", func(_ i034, _ *Debugging) { called["InjectorChainDebugging"]++ })), // included
				Provide("base-collection-2", func(_ i031, _ i034) i035 { called["base-collection-2"]++; return nil }),         // included
				Provide("WriteJSON", func(inner func() i036, _ i035, _ i034) {
					called["WriteJSON"]++
					inner()
				}), // included
				Desired(Provide("IfError", func(inner func() (i036, s037), _ i034, _ i035) i036 {
					called["IfError"]++
					inner()
					return nil
				})), // included
				Provide("AsErrors", func(inner func() error, _ i034, _ i035) s037 {
					called["AsErrors"]++
					inner()
					return 0
				}), // included
				Provide("SaveRequest", func(_ i031, _ s032) (TerminalError, s038) { called["SaveRequest"]++; return nil, 0 }),            // included
				NotCacheable(MustConsume(Provide("DBOpen", func(_ s032) (i044, TerminalError) { called["DBOpen"]++; return nil, nil }))), // included
				MustConsume(Provide("DBClose", func(inner func(i044), _ i044) {
					called["DBClose"]++
					inner(nil)
				})), // included
				Provide("TimeTravelHeader", func() s051 { called["TimeTravelHeader"]++; return 0 }),
				MustConsume(Provide("ParseClientQuote", func(_ i034, _ s038) (TerminalError, s066) { called["ParseClientQuote"]++; return nil, 0 })), // included
				Provide("ConvertCardToken", func(_ i034, _ s066) (s066, TerminalError) { called["ConvertCardToken"]++; return 0, nil }),
				MustConsume(Provide("Tx", func(inner func(i045, i046) error, _ s032, _ i044, _ i034, _ i035) error {
					called["Tx"]++
					inner(nil, nil)
					return nil
				})),
				Provide("ConsumeTxDone", func(_ i046) { called["ConsumeTxDone"]++ }),
				MustConsume(Provide("ParentTx", func(_ i045) i047 { called["ParentTx"]++; return nil })),
				Provide("GetSessionID", func(_ i034, _ s032) (TerminalError, s053) { called["GetSessionID"]++; return nil, 0 }), // included
				Provide("AdvisoryLockQuote", func(inner func(s054) error, _ i034, _ i044, _ s053) error {
					called["AdvisoryLockQuote"]++
					inner(0)
					return nil
				}), // included
				Provide("LoadSavedQuote", func(_ i034, _ s054, _ s053) (TerminalError, s067) { called["LoadSavedQuote"]++; return nil, 0 }), // included
				Desired(MustConsume(Provide("PurchaseFailureAlerter", func(inner func(s068) error, _ i034, _ s067, _ s053) {
					called["PurchaseFailureAlerter"]++
					inner(0)
				}))),
				Provide("VariationsRuleChecker", func(_ i034, _ s062, _ s053, _ s054) s069 { called["VariationsRuleChecker"]++; return 0 }), // included
				Provide("ReassembleQuote", func(_ i034, _ s066, _ s067, _ s061) (TerminalError, s070, s071) {
					called["ReassembleQuote"]++
					return nil, 0, 0
				}), // included
				Provide("CheckAndUpdateQuote", func(_ s032, _ i034, _ s069, _ s065, _ s064, _ s063, _ s070, _ s053, _ s054, _ s071) (TerminalError, s056, s055) {
					called["CheckAndUpdateQuote"]++
					return nil, 0, 0
				}),
				Provide("endpoint-0", func(inner func() s058) i036 {
					called["endpoint-0"]++
					inner()
					return nil
				}), // included
				Required(Provide("endpoint-1", func(_ i034, _ s032, _ i035, _ s038, _ i044, _ s060) (s058, error) {
					called["endpoint-1"]++
					return 0, nil
				})), // included
			).Bind(&invoker, &initer))
		initer()
		invoker(nil, 0)
		assert.Equal(t, 0, called["ReassembleQuote"])
	})
}

func TestRegression9(t *testing.T) {
	type i003 interface {
		x003()
	}
	type s004 int
	type i005 interface {
		x005()
	}
	type s006 int
	type s009 int
	type i011 interface {
		x011()
	}
	type i013 interface {
		x013()
	}
	type i015 interface {
		x015()
	}
	type s017 int
	type s018 int
	type s020 int
	type i021 interface {
		x021()
	}
	type s022 int
	type s019 int
	type s023 int
	type s024 int
	type s025 int
	type s026 int
	type s027 int
	type s028 int
	type s029 int
	type s030 int
	type s031 int
	type s032 int
	type s033 int
	type s034 int
	type s035 int
	type i036 interface {
		x036()
	}
	type s037 int
	type s038 int
	type s039 int
	type s040 int
	type s041 int
	type s043 int
	type s044 int
	type s045 int
	type s046 int
	type s047 int
	type s048 int
	type s049 int
	type s050 int
	type s051 int
	type s007 int
	type s008 int
	type s010 int
	type s012 int
	type s014 int
	type s016 int
	type i052 interface {
		x052()
	}
	type s054 int

	wrapTest(t, func(t *testing.T) {
		called := make(map[string]int)
		var invoker func() error
		err := Sequence("regression",
			Provide("Run()error", func() TerminalError { called["Run()error"]++; return nil }),
			Provide("TCP-0", func() i003 { called["TCP-0"]++; return nil }),
			Provide("TCP-1", func(inner func() error, _ s004) {
				called["TCP-1"]++
				inner()
			}),
			Provide("TCP-2", func() i005 { called["TCP-2"]++; return nil }),
			Provide("TCP-3", func(_ i005) i005 { called["TCP-3"]++; return nil }),
			Shun(Provide("integration-before-user-0", func() s006 { called["integration-before-user-0"]++; return 0 })),
			Shun(Provide("integration-before-user-1", func() s009 { called["integration-before-user-1"]++; return 0 })),
			Provide("base-chain-0", func(_ i003) i011 { called["base-chain-0"]++; return nil }),
			Provide("base-chain-1", func(_ i003) i013 { called["base-chain-1"]++; return nil }),
			Provide("base-chain-2", func() i015 { called["base-chain-2"]++; return nil }),
			Provide("base-chain-3", func(_ i003, _ s006, _ i011) s017 { called["base-chain-3"]++; return 0 }),
			Provide("base-chain-4", func(_ i003, _ i011, _ s018, _ i015) s020 { called["base-chain-4"]++; return 0 }),
			Provide("server-chain-0", func(_ i003, _ i011) i021 { called["server-chain-0"]++; return nil }),
			Provide("server-chain-1", func() s022 { called["server-chain-1"]++; return 0 }),
			Provide("server-chain-2", func() s019 { called["server-chain-2"]++; return 0 }),
			Shun(Provide("server-chain-3", func() s023 { called["server-chain-3"]++; return 0 })),
			Provide("server-chain-4", func(_ i003, _ s023, _ i015, _ i011) s024 { called["server-chain-4"]++; return 0 }),
			Provide("server-chain-5", func(_ i011) s025 { called["server-chain-5"]++; return 0 }),
			Shun(Provide("server-chain-6", func() s026 { called["server-chain-6"]++; return 0 })),
			Provide("server-chain-7", func(_ i003, _ i015, _ s026, _ i011) s027 { called["server-chain-7"]++; return 0 }),
			Shun(Provide("server-chain-8", func() s028 { called["server-chain-8"]++; return 0 })),
			Provide("server-chain-9", func(_ i003, _ i015, _ s028, _ i011) s029 { called["server-chain-9"]++; return 0 }),
			Shun(Provide("server-chain-10", func() s030 { called["server-chain-10"]++; return 0 })),
			Provide("server-chain-11", func(_ i011) s031 { called["server-chain-11"]++; return 0 }),
			Provide("server-chain-12", func(_ i011) s032 { called["server-chain-12"]++; return 0 }),
			Shun(Provide("server-chain-13", func() s033 { called["server-chain-13"]++; return 0 })),
			Provide("server-chain-14", func(_ s020, _ i011, _ s033) s034 { called["server-chain-14"]++; return 0 }),
			Shun(Provide("server-chain-15", func() s035 { called["server-chain-15"]++; return 0 })),
			Provide("server-chain-16", func(_ i003, _ i011, _ i021, _ s035) i036 { called["server-chain-16"]++; return nil }),
			Provide("server-chain-17", func(_ i005, _ i003, _ s037, _ s022, _ i013) s038 { called["server-chain-17"]++; return 0 }),
			Provide("server-chain-18", func(_ i003) s039 { called["server-chain-18"]++; return 0 }),
			Provide("server-chain-19", func(_ i003) s040 { called["server-chain-19"]++; return 0 }),
			Provide("server-chain-20", func(_ s017, _ i013) s041 { called["server-chain-20"]++; return 0 }),
			Provide("server-chain-21", func(_ s041, _ s020, _ s039, _ s040) s043 { called["server-chain-21"]++; return 0 }),
			Provide("server-chain-22", func(_ s020, _ s039, _ s040, _ i011, _ s017) s044 { called["server-chain-22"]++; return 0 }),
			Provide("integration-before-user-4", func(_ i011, _ s044, _ i013, _ s017, _ s038, _ s025, _ i036, _ s020, _ s024, _ s034, _ s045, _ s029, _ s030, _ s031, _ s032) s046 {
				called["integration-before-user-4"]++
				return 0
			}),
			Provide("integration-before-user-5", func(_ i011, _ s044, _ i013, _ s017, _ s038, _ s025, _ i036, _ s020, _ s024, _ s045, _ s034, _ s029, _ s030, _ s031, _ s032) s047 {
				called["integration-before-user-5"]++
				return 0
			}),
			Provide("environments-0", func(_ i003, _ i011) s048 { called["environments-0"]++; return 0 }),
			Provide("environments-1", func(_ i005, _ s017, _ s046, _ s047, _ s048, _ s006) s049 { called["environments-1"]++; return 0 }),
			Provide("environments-2", func(_ s048, _ s019, _ s020) s050 { called["environments-2"]++; return 0 }),
			Provide("environments-3", func(_ i005, _ s048, _ s049, _ s050) s051 { called["environments-3"]++; return 0 }),
			Provide("integration-before-user-7", func() s019 { called["integration-before-user-7"]++; return 0 }),
			Provide("integration-user-and-more-0", func(_ i005) s007 { called["integration-user-and-more-0"]++; return 0 }),
			Provide("public-client-0", func(_ i005, _ s007) s008 { called["public-client-0"]++; return 0 }),
			Provide("public-client-1", func(_ s008) s010 { called["public-client-1"]++; return 0 }),
			Provide("integration-user-and-more-2", func(_ i005, _ i003, _ s010, _ s009) s012 { called["integration-user-and-more-2"]++; return 0 }),
			Provide("integration-user-and-more-3", func(_ s012) s014 { called["integration-user-and-more-3"]++; return 0 }),
			Provide("integration-user-and-more-4", func(_ i005, _ i003, _ s008) s016 { called["integration-user-and-more-4"]++; return 0 }),
			Provide("cluster-0", func(_ i003) i052 { called["cluster-0"]++; return nil }),
			Provide("revs-0", func(_ i003) { called["revs-0"]++ }),
			Reorder(Provide("revs-1", func() s009 { called["revs-1"]++; return 0 })),
			Provide("revs-2", func(_ i005) (s045, error) { called["revs-2"]++; return 0, nil }),
			Required(Provide("revs-3", func(_ i005, _ i003, _ s008, _ s045, _ s012) { called["revs-3"]++ })),
			Provide("revs-4", func(_ i005, _ i003, _ s019) s054 { called["revs-4"]++; return 0 }),
			Provide("user-chain-1", func(_ i005, _ i003, _ s019, _ s054) { called["user-chain-1"]++ }),
			Provide("user-chain-2", func(_ i005, _ i003, _ s019, _ s054) { called["user-chain-2"]++ }),
			Shun(NonFinal(Provide("TCP-6", func(inner func()) error {
				called["TCP-6"]++
				inner()
				return nil
			}))),
			Provide("user-chain-3", func(_ i005, _ s054, _ s014, _ s019, _ s012) { called["user-chain-3"]++ }),
		).Bind(&invoker, nil)
		//nolint:testifylint
		if !assert.NoError(t, err, "bind error") {
			t.Log(DetailedError(err))
		}
		// invoker()
	})
}

func TestClusterRegression(t *testing.T) {
	genTest := func(clustering bool) func(*testing.T) {
		return func(t *testing.T) {
			var nn, nd, ndd, tc bool
			seq1 := []interface{}{
				Provide("needed", func() string { return "foo" }),
				Provide("not-needed", func() int64 {
					nn = true
					return 0
				}),
			}
			seq2 := []interface{}{
				Provide("normally-desired", func(string) {
					nd = true
				}),
				Provide("normally-desired too", func(string) {}),
			}
			seq3 := []interface{}{
				Provide("normally-desired in degenerate cluster", func(string) {
					ndd = true
				}),
			}
			test := func(s string) {
				assert.Equal(t, "foo", s)
				tc = true
			}
			if clustering {
				require.NoError(t, Run(t.Name(), Cluster("s1", seq1...), Cluster("s2", seq2...), Cluster("s3", seq3...), test))
			} else {
				require.NoError(t, Run(t.Name(), Sequence("s1", seq1...), Sequence("s2", seq2...), Sequence("s3", seq3...), test))
			}
			require.True(t, tc, "tc")
			assert.Equal(t, clustering, nn, "nn")
			assert.Equal(t, !clustering, nd, "nd")
			assert.True(t, ndd, "ndd")
		}
	}
	namedWrapTest(t, " cluster", genTest(true))
	namedWrapTest(t, " sequence", genTest(false))
}
