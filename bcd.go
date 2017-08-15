// Copyright (c) 2017 Niko Carpenter
// Use of this source code is governed by the MIT License,
// which can be found in the LICENSE file.

package clocktower

import (
	"github.com/pkg/errors"
)

// A bit in the digital time code can either be 0, 1, or a marker.
const (
	bit0 byte = iota
	bit1
	bitMarker
	bitNone
)

// A fieldDef holds the information needed to encode a single value into binary coded decimal.
type fieldDef struct {
	label   string
	weights []int
	maxVal  int
}

// newFieldDef creates a new field definition.
func newFieldDef(label string, weights ...int) fieldDef {
	fd := fieldDef{label, weights, 0}
	for _, w := range weights {
		fd.maxVal += w
	}

	return fd
}

// a bCDEncoder holds the information needed to encode a slice of values into a buffer of binary encoded decimal bytes.
type bCDEncoder struct {
	fieldDefs []fieldDef
	outSize   int
}

// newBCDEncoder initializes a binary coded decimal encoder, where each fieldDef  defines the encoding for one value.
// Each weight in a fieldDef consumes one element in the buffer passed to bCDEncoder.encode.
// This means one bit in the output consumes one byte in the resulting buffer. This is 8x larger than packing 8 bits into 1 byte,
// but it has the advantage of being iterable, and able to store other values besides 0 and 1.
//
// A weight cannot be negative, and with the exception of weight = 0,
// a fieldDef's weights must be sorted in ascending order.
// Valid fieldDef.weights: [1 2 4 8 0 10 20 40 80]
// Invalid: [2 8 1 4 20 10 80 40]
//
// A 0 weight will leave the corresponding element in the buffer untouched.
func newBCDEncoder(fieldDefs []fieldDef) (*bCDEncoder, error) {
	// Verify that weights are sorted before continuing, and calculate the total output size.
	outSize := 0
	for i := range fieldDefs {
		weights := fieldDefs[i].weights
		outSize += len(weights)
		lastW := 0
		for _, w := range weights {
			if w == 0 {
				continue
			}
			if w < lastW {
				return nil, errors.Errorf("Weights must be >= 0, and sorted in ascending order; got %v for fieldDef %s", weights, fieldDefs[i].label)
			}
			lastW = w
		}
	}

	return &bCDEncoder{fieldDefs, outSize}, nil
}

// encode encodes a slice of values into outBuff.
func (b *bCDEncoder) encode(outBuff []byte, vals []int) error {
	if len(vals) != len(b.fieldDefs) {
		return errors.Errorf("The number of values to encode (%d) does not equal the number of fieldDefs (%d)", len(vals), len(b.fieldDefs))
	}
	if b.outSize > len(outBuff) {
		return errors.Errorf("The encoded output is %d bytes, but the provided buffer is only %d bytes", b.outSize, len(outBuff))
	}

	seek := 0
	for i, v := range vals {
		if v < 0 {
			return errors.Errorf("Only positive integers can be encoded; got %d for field %s", v, b.fieldDefs[i].label)
		}
		if v > b.fieldDefs[i].maxVal {
			return errors.Errorf("The value %d is too large to be encoded for the field %s", v, b.fieldDefs[i].label)
		}

		weights := b.fieldDefs[i].weights
		fSize := len(weights)
		for j := fSize - 1; j >= 0; j-- {
			if weights[j] == 0 {
				continue
			}
			if v >= weights[j] {
				v -= weights[j]
				outBuff[seek+j] = bit1
			} else {
				outBuff[seek+j] = bit0
			}
		}
		seek += fSize
	}

	return nil
}
