/*
 *  FillPDF - Fill PDF forms
 *  Copyright 2022 Karel Bilek
 *  Copyright DesertBit
 *  Author: Roland Singer
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 */

package fillpdf

import (
	"bytes"
	"fmt"
	"os/exec"
)

func runCommandInPath(dir, name string, args ...string) ([]byte, error) {
	// Create the command.
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	b, err := cmd.Output()
	if err == nil {
		return b, nil
	}
	exitErr, isTyped := err.(*exec.ExitError)
	if !isTyped {
		return nil, err
	}
	return nil, fmt.Errorf("%w: %s", err, exitErr.Stderr)
}

func runCommandInPathWithStdin(in []byte, dir, name string, args ...string) ([]byte, error) {
	// Create the command.
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdin = bytes.NewReader(in)
	b, err := cmd.Output()
	if err == nil {
		return b, nil
	}
	exitErr, isTyped := err.(*exec.ExitError)
	if !isTyped {
		return nil, err
	}
	return nil, fmt.Errorf("%w: %s", err, exitErr.Stderr)
}
