// Copyright 2015 CoreOS, Inc.
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

package retry

import (
	"fmt"
	"time"
)

// Retry calls function f until it has been called attemps times, or succeeds.
// Retry delays for delay between calls of f. If f does not succeed after
// attempts calls, the error from the last call is returned.
func Retry(attempts int, delay time.Duration, f func() error) error {
	return RetryConditional(attempts, delay, func(_ error) bool { return true }, f)
}

// RetryConditional calls function f until it has been called attemps times, or succeeds.
// Retry delays for delay between calls of f. If f does not succeed after
// attempts calls, the error from the last call is returned.
// If shouldRetry returns false on the error generated, RetryConditional stops immediately
// and returns the error
func RetryConditional(attempts int, delay time.Duration, shouldRetry func(err error) bool, f func() error) error {
	var err error

	for i := 0; i < attempts; i++ {
		err = f()
		if err == nil || !shouldRetry(err) {
			break
		}

		if i < attempts-1 {
			time.Sleep(delay)
		}
	}

	return err
}

// RetryUntilTimeout calls function f until it succeeds or until
// the given timeout is reached. It will wait a given amount of time
// between each try based on the given delay.
func RetryUntilTimeout(timeout, delay time.Duration, f func() error) error {
	after := time.After(timeout)
	for {
		select {
		case <-after:
			return fmt.Errorf("time limit exceeded")
		default:
		}
		err := f()
		if err == nil {
			break
		}
		time.Sleep(delay)
	}
	return nil
}

func WaitUntilReady(timeout, delay time.Duration, checkFunction func() (bool, error)) error {
	after := time.After(timeout)
	for {
		select {
		case <-after:
			return fmt.Errorf("time limit exceeded")
		default:
		}

		done, err := checkFunction()
		if err != nil {
			return err
		}
		if done {
			break
		}
		time.Sleep(delay)
	}
	return nil
}
