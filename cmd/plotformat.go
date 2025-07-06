package cmd

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
)

const (
	PlotVersion = int64(1)
)

type PlotHeader struct {
	Version int64
	K       int32
	Entries []AddressOffsetPair
}

type AddressOffsetPair struct {
	Address    [AddressSize]byte    // Ethereum-style address
	Identifier [IdentifierSize]byte // SHAKE128 identifier for matching
	Offset     int32                // Offset to private key in file
}

func (h *PlotHeader) WriteTo(w io.Writer) error {
	if err := binary.Write(w, binary.LittleEndian, h.Version); err != nil {
		return fmt.Errorf("failed to write version: %w", err)
	}
	
	if err := binary.Write(w, binary.LittleEndian, h.K); err != nil {
		return fmt.Errorf("failed to write K value: %w", err)
	}
	
	entryCount := int32(len(h.Entries))
	if err := binary.Write(w, binary.LittleEndian, entryCount); err != nil {
		return fmt.Errorf("failed to write entry count: %w", err)
	}
	
	for i, entry := range h.Entries {
		if err := binary.Write(w, binary.LittleEndian, entry.Address); err != nil {
			return fmt.Errorf("failed to write address %d: %w", i, err)
		}
		if err := binary.Write(w, binary.LittleEndian, entry.Identifier); err != nil {
			return fmt.Errorf("failed to write identifier %d: %w", i, err)
		}
		if err := binary.Write(w, binary.LittleEndian, entry.Offset); err != nil {
			return fmt.Errorf("failed to write offset %d: %w", i, err)
		}
	}
	
	return nil
}

func (h *PlotHeader) ReadFrom(r io.Reader) error {
	if err := binary.Read(r, binary.LittleEndian, &h.Version); err != nil {
		return fmt.Errorf("failed to read version: %w", err)
	}
	
	if err := binary.Read(r, binary.LittleEndian, &h.K); err != nil {
		return fmt.Errorf("failed to read K value: %w", err)
	}
	
	var entryCount int32
	if err := binary.Read(r, binary.LittleEndian, &entryCount); err != nil {
		return fmt.Errorf("failed to read entry count: %w", err)
	}
	
	h.Entries = make([]AddressOffsetPair, entryCount)
	for i := 0; i < int(entryCount); i++ {
		if err := binary.Read(r, binary.LittleEndian, &h.Entries[i].Address); err != nil {
			return fmt.Errorf("failed to read address %d: %w", i, err)
		}
		if err := binary.Read(r, binary.LittleEndian, &h.Entries[i].Identifier); err != nil {
			return fmt.Errorf("failed to read identifier %d: %w", i, err)
		}
		if err := binary.Read(r, binary.LittleEndian, &h.Entries[i].Offset); err != nil {
			return fmt.Errorf("failed to read offset %d: %w", i, err)
		}
	}
	
	return nil
}

func (h *PlotHeader) Size() int {
	return 8 + 4 + 4 + len(h.Entries)*(AddressSize+IdentifierSize+4)
}

func (aop *AddressOffsetPair) AddressHex() string {
	return hex.EncodeToString(aop.Address[:])
}

func (aop *AddressOffsetPair) IdentifierHex() string {
	return hex.EncodeToString(aop.Identifier[:])
}