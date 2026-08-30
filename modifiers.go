package pgxmock

// This file gives every expectation type its own copy of the CallModifier
// methods, returning the concrete type rather than the CallModifier interface.
// Without them a chain had to be written in one particular order, because the
// interface does not carry the type-specific builders:
//
//	mock.ExpectQuery(sql).WillReturnRows(rows).Times(2) // compiled
//	mock.ExpectQuery(sql).Times(2).WillReturnRows(rows) // did not
//
// The bodies are mechanical: each one delegates to the embedded
// commonExpectation and returns its receiver.

import "time"

// --- ExpectedBatch ---

// Maybe allows the expected method call to be optional.
// Not calling an optional method will not cause an error while asserting expectations.
func (e *ExpectedBatch) Maybe() *ExpectedBatch {
	e.commonExpectation.Maybe()
	return e
}

// Times indicates that the expected method should only fire the indicated number of times.
// Zero value is ignored and means the same as one.
func (e *ExpectedBatch) Times(n uint) *ExpectedBatch {
	e.commonExpectation.Times(n)
	return e
}

// WillDelayFor allows to specify duration for which it will delay result.
// May be used together with Context.
func (e *ExpectedBatch) WillDelayFor(duration time.Duration) *ExpectedBatch {
	e.commonExpectation.WillDelayFor(duration)
	return e
}

// WillReturnError allows to set an error for the expected method.
func (e *ExpectedBatch) WillReturnError(err error) *ExpectedBatch {
	e.commonExpectation.WillReturnError(err)
	return e
}

// WillPanic allows to force the expected method to panic.
func (e *ExpectedBatch) WillPanic(v any) *ExpectedBatch {
	e.commonExpectation.WillPanic(v)
	return e
}

// --- ExpectedBegin ---

// Maybe allows the expected method call to be optional.
// Not calling an optional method will not cause an error while asserting expectations.
func (e *ExpectedBegin) Maybe() *ExpectedBegin {
	e.commonExpectation.Maybe()
	return e
}

// Times indicates that the expected method should only fire the indicated number of times.
// Zero value is ignored and means the same as one.
func (e *ExpectedBegin) Times(n uint) *ExpectedBegin {
	e.commonExpectation.Times(n)
	return e
}

// WillDelayFor allows to specify duration for which it will delay result.
// May be used together with Context.
func (e *ExpectedBegin) WillDelayFor(duration time.Duration) *ExpectedBegin {
	e.commonExpectation.WillDelayFor(duration)
	return e
}

// WillReturnError allows to set an error for the expected method.
func (e *ExpectedBegin) WillReturnError(err error) *ExpectedBegin {
	e.commonExpectation.WillReturnError(err)
	return e
}

// WillPanic allows to force the expected method to panic.
func (e *ExpectedBegin) WillPanic(v any) *ExpectedBegin {
	e.commonExpectation.WillPanic(v)
	return e
}

// --- ExpectedClose ---

// Maybe allows the expected method call to be optional.
// Not calling an optional method will not cause an error while asserting expectations.
func (e *ExpectedClose) Maybe() *ExpectedClose {
	e.commonExpectation.Maybe()
	return e
}

// Times indicates that the expected method should only fire the indicated number of times.
// Zero value is ignored and means the same as one.
func (e *ExpectedClose) Times(n uint) *ExpectedClose {
	e.commonExpectation.Times(n)
	return e
}

// WillDelayFor allows to specify duration for which it will delay result.
// May be used together with Context.
func (e *ExpectedClose) WillDelayFor(duration time.Duration) *ExpectedClose {
	e.commonExpectation.WillDelayFor(duration)
	return e
}

// WillReturnError allows to set an error for the expected method.
func (e *ExpectedClose) WillReturnError(err error) *ExpectedClose {
	e.commonExpectation.WillReturnError(err)
	return e
}

// WillPanic allows to force the expected method to panic.
func (e *ExpectedClose) WillPanic(v any) *ExpectedClose {
	e.commonExpectation.WillPanic(v)
	return e
}

// --- ExpectedCommit ---

// Maybe allows the expected method call to be optional.
// Not calling an optional method will not cause an error while asserting expectations.
func (e *ExpectedCommit) Maybe() *ExpectedCommit {
	e.commonExpectation.Maybe()
	return e
}

// Times indicates that the expected method should only fire the indicated number of times.
// Zero value is ignored and means the same as one.
func (e *ExpectedCommit) Times(n uint) *ExpectedCommit {
	e.commonExpectation.Times(n)
	return e
}

// WillDelayFor allows to specify duration for which it will delay result.
// May be used together with Context.
func (e *ExpectedCommit) WillDelayFor(duration time.Duration) *ExpectedCommit {
	e.commonExpectation.WillDelayFor(duration)
	return e
}

// WillReturnError allows to set an error for the expected method.
func (e *ExpectedCommit) WillReturnError(err error) *ExpectedCommit {
	e.commonExpectation.WillReturnError(err)
	return e
}

// WillPanic allows to force the expected method to panic.
func (e *ExpectedCommit) WillPanic(v any) *ExpectedCommit {
	e.commonExpectation.WillPanic(v)
	return e
}

// --- ExpectedCopyFrom ---

// Maybe allows the expected method call to be optional.
// Not calling an optional method will not cause an error while asserting expectations.
func (e *ExpectedCopyFrom) Maybe() *ExpectedCopyFrom {
	e.commonExpectation.Maybe()
	return e
}

// Times indicates that the expected method should only fire the indicated number of times.
// Zero value is ignored and means the same as one.
func (e *ExpectedCopyFrom) Times(n uint) *ExpectedCopyFrom {
	e.commonExpectation.Times(n)
	return e
}

// WillDelayFor allows to specify duration for which it will delay result.
// May be used together with Context.
func (e *ExpectedCopyFrom) WillDelayFor(duration time.Duration) *ExpectedCopyFrom {
	e.commonExpectation.WillDelayFor(duration)
	return e
}

// WillReturnError allows to set an error for the expected method.
func (e *ExpectedCopyFrom) WillReturnError(err error) *ExpectedCopyFrom {
	e.commonExpectation.WillReturnError(err)
	return e
}

// WillPanic allows to force the expected method to panic.
func (e *ExpectedCopyFrom) WillPanic(v any) *ExpectedCopyFrom {
	e.commonExpectation.WillPanic(v)
	return e
}

// --- ExpectedDeallocate ---

// Maybe allows the expected method call to be optional.
// Not calling an optional method will not cause an error while asserting expectations.
func (e *ExpectedDeallocate) Maybe() *ExpectedDeallocate {
	e.commonExpectation.Maybe()
	return e
}

// Times indicates that the expected method should only fire the indicated number of times.
// Zero value is ignored and means the same as one.
func (e *ExpectedDeallocate) Times(n uint) *ExpectedDeallocate {
	e.commonExpectation.Times(n)
	return e
}

// WillDelayFor allows to specify duration for which it will delay result.
// May be used together with Context.
func (e *ExpectedDeallocate) WillDelayFor(duration time.Duration) *ExpectedDeallocate {
	e.commonExpectation.WillDelayFor(duration)
	return e
}

// WillReturnError allows to set an error for the expected method.
func (e *ExpectedDeallocate) WillReturnError(err error) *ExpectedDeallocate {
	e.commonExpectation.WillReturnError(err)
	return e
}

// WillPanic allows to force the expected method to panic.
func (e *ExpectedDeallocate) WillPanic(v any) *ExpectedDeallocate {
	e.commonExpectation.WillPanic(v)
	return e
}

// --- ExpectedExec ---

// Maybe allows the expected method call to be optional.
// Not calling an optional method will not cause an error while asserting expectations.
func (e *ExpectedExec) Maybe() *ExpectedExec {
	e.commonExpectation.Maybe()
	return e
}

// Times indicates that the expected method should only fire the indicated number of times.
// Zero value is ignored and means the same as one.
func (e *ExpectedExec) Times(n uint) *ExpectedExec {
	e.commonExpectation.Times(n)
	return e
}

// WillDelayFor allows to specify duration for which it will delay result.
// May be used together with Context.
func (e *ExpectedExec) WillDelayFor(duration time.Duration) *ExpectedExec {
	e.commonExpectation.WillDelayFor(duration)
	return e
}

// WillReturnError allows to set an error for the expected method.
func (e *ExpectedExec) WillReturnError(err error) *ExpectedExec {
	e.commonExpectation.WillReturnError(err)
	return e
}

// WillPanic allows to force the expected method to panic.
func (e *ExpectedExec) WillPanic(v any) *ExpectedExec {
	e.commonExpectation.WillPanic(v)
	return e
}

// --- ExpectedPing ---

// Maybe allows the expected method call to be optional.
// Not calling an optional method will not cause an error while asserting expectations.
func (e *ExpectedPing) Maybe() *ExpectedPing {
	e.commonExpectation.Maybe()
	return e
}

// Times indicates that the expected method should only fire the indicated number of times.
// Zero value is ignored and means the same as one.
func (e *ExpectedPing) Times(n uint) *ExpectedPing {
	e.commonExpectation.Times(n)
	return e
}

// WillDelayFor allows to specify duration for which it will delay result.
// May be used together with Context.
func (e *ExpectedPing) WillDelayFor(duration time.Duration) *ExpectedPing {
	e.commonExpectation.WillDelayFor(duration)
	return e
}

// WillReturnError allows to set an error for the expected method.
func (e *ExpectedPing) WillReturnError(err error) *ExpectedPing {
	e.commonExpectation.WillReturnError(err)
	return e
}

// WillPanic allows to force the expected method to panic.
func (e *ExpectedPing) WillPanic(v any) *ExpectedPing {
	e.commonExpectation.WillPanic(v)
	return e
}

// --- ExpectedPrepare ---

// Maybe allows the expected method call to be optional.
// Not calling an optional method will not cause an error while asserting expectations.
func (e *ExpectedPrepare) Maybe() *ExpectedPrepare {
	e.commonExpectation.Maybe()
	return e
}

// Times indicates that the expected method should only fire the indicated number of times.
// Zero value is ignored and means the same as one.
func (e *ExpectedPrepare) Times(n uint) *ExpectedPrepare {
	e.commonExpectation.Times(n)
	return e
}

// WillDelayFor allows to specify duration for which it will delay result.
// May be used together with Context.
func (e *ExpectedPrepare) WillDelayFor(duration time.Duration) *ExpectedPrepare {
	e.commonExpectation.WillDelayFor(duration)
	return e
}

// WillReturnError allows to set an error for the expected method.
func (e *ExpectedPrepare) WillReturnError(err error) *ExpectedPrepare {
	e.commonExpectation.WillReturnError(err)
	return e
}

// WillPanic allows to force the expected method to panic.
func (e *ExpectedPrepare) WillPanic(v any) *ExpectedPrepare {
	e.commonExpectation.WillPanic(v)
	return e
}

// --- ExpectedQuery ---

// Maybe allows the expected method call to be optional.
// Not calling an optional method will not cause an error while asserting expectations.
func (e *ExpectedQuery) Maybe() *ExpectedQuery {
	e.commonExpectation.Maybe()
	return e
}

// Times indicates that the expected method should only fire the indicated number of times.
// Zero value is ignored and means the same as one.
func (e *ExpectedQuery) Times(n uint) *ExpectedQuery {
	e.commonExpectation.Times(n)
	return e
}

// WillDelayFor allows to specify duration for which it will delay result.
// May be used together with Context.
func (e *ExpectedQuery) WillDelayFor(duration time.Duration) *ExpectedQuery {
	e.commonExpectation.WillDelayFor(duration)
	return e
}

// WillReturnError allows to set an error for the expected method.
func (e *ExpectedQuery) WillReturnError(err error) *ExpectedQuery {
	e.commonExpectation.WillReturnError(err)
	return e
}

// WillPanic allows to force the expected method to panic.
func (e *ExpectedQuery) WillPanic(v any) *ExpectedQuery {
	e.commonExpectation.WillPanic(v)
	return e
}

// --- ExpectedReset ---

// Maybe allows the expected method call to be optional.
// Not calling an optional method will not cause an error while asserting expectations.
func (e *ExpectedReset) Maybe() *ExpectedReset {
	e.commonExpectation.Maybe()
	return e
}

// Times indicates that the expected method should only fire the indicated number of times.
// Zero value is ignored and means the same as one.
func (e *ExpectedReset) Times(n uint) *ExpectedReset {
	e.commonExpectation.Times(n)
	return e
}

// WillDelayFor allows to specify duration for which it will delay result.
// May be used together with Context.
func (e *ExpectedReset) WillDelayFor(duration time.Duration) *ExpectedReset {
	e.commonExpectation.WillDelayFor(duration)
	return e
}

// WillReturnError allows to set an error for the expected method.
func (e *ExpectedReset) WillReturnError(err error) *ExpectedReset {
	e.commonExpectation.WillReturnError(err)
	return e
}

// WillPanic allows to force the expected method to panic.
func (e *ExpectedReset) WillPanic(v any) *ExpectedReset {
	e.commonExpectation.WillPanic(v)
	return e
}

// --- ExpectedRollback ---

// Maybe allows the expected method call to be optional.
// Not calling an optional method will not cause an error while asserting expectations.
func (e *ExpectedRollback) Maybe() *ExpectedRollback {
	e.commonExpectation.Maybe()
	return e
}

// Times indicates that the expected method should only fire the indicated number of times.
// Zero value is ignored and means the same as one.
func (e *ExpectedRollback) Times(n uint) *ExpectedRollback {
	e.commonExpectation.Times(n)
	return e
}

// WillDelayFor allows to specify duration for which it will delay result.
// May be used together with Context.
func (e *ExpectedRollback) WillDelayFor(duration time.Duration) *ExpectedRollback {
	e.commonExpectation.WillDelayFor(duration)
	return e
}

// WillReturnError allows to set an error for the expected method.
func (e *ExpectedRollback) WillReturnError(err error) *ExpectedRollback {
	e.commonExpectation.WillReturnError(err)
	return e
}

// WillPanic allows to force the expected method to panic.
func (e *ExpectedRollback) WillPanic(v any) *ExpectedRollback {
	e.commonExpectation.WillPanic(v)
	return e
}
