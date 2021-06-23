/*
SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors

SPDX-License-Identifier: Apache-2.0
*/
package fake

import (
	"bytes"
	"sync"

	"k8s.io/cli-runtime/pkg/genericclioptions"
)

// NewIOStreams returns a valid IOStreams and in, out, errout buffers for unit tests
func NewIOStreams() (genericclioptions.IOStreams, *SafeBytesBuffer, *SafeBytesBuffer, *SafeBytesBuffer) {
	in := &SafeBytesBuffer{}
	out := &SafeBytesBuffer{}
	errOut := &SafeBytesBuffer{}

	return genericclioptions.IOStreams{
		In:     in,
		Out:    out,
		ErrOut: errOut,
	}, in, out, errOut
}

// SafeBytesBuffer is a bytes.Buffer that is safe for use in multiple
// goroutines.
type SafeBytesBuffer struct {
	buffer bytes.Buffer
	mutex  sync.Mutex
}

// Read reads the next len(p) bytes from the buffer or until the buffer
// is drained. The return value n is the number of bytes read. If the
// buffer has no data to return, err is io.EOF (unless len(p) is zero);
// otherwise it is nil.
func (s *SafeBytesBuffer) Read(p []byte) (n int, err error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	return s.buffer.Read(p)
}

// Write appends the contents of p to the buffer, growing the buffer as
// needed. The return value n is the length of p; err is always nil. If the
// buffer becomes too large, Write will panic with ErrTooLarge.
func (s *SafeBytesBuffer) Write(p []byte) (n int, err error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	return s.buffer.Write(p)
}

// String returns the contents of the unread portion of the buffer
// as a string. If the Buffer is a nil pointer, it returns "<nil>".
//
// To build strings more efficiently, see the strings.Builder type.
func (s *SafeBytesBuffer) String() string {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	return s.buffer.String()
}
