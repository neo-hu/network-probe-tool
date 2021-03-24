package icmp

import (
	"math"
	"os"
)

// ICMP Seq max MaxUint16, 如果发送太多的包导致seq不够用

type SeqPool struct {
	ident        map[uint16]map[uint16]interface{}
	currentSeq   uint16
	currentIdent uint16
}

func NewSeqPool(ident uint16) *SeqPool {
	if ident <= 0 {
		ident = uint16(os.Getpid() & 0xFFFF)
	}
	s := &SeqPool{
		ident:        map[uint16]map[uint16]interface{}{},
		currentIdent: ident,
	}
	return s
}

func (s *SeqPool) Apply(v interface{}) (uint16, uint16) {
	seq := s.currentSeq
	if s.currentSeq >= math.MaxUint16 {
		s.currentSeq = 0
		s.currentIdent += 1
	} else {
		s.currentSeq += 1
	}
	i, ok := s.ident[s.currentIdent]
	if !ok {
		i = map[uint16]interface{}{}
		s.ident[s.currentIdent] = i
	}
	i[seq] = v
	return s.currentIdent, seq
}

func (s *SeqPool) Free(ident uint16, seq uint16) interface{} {
	i, ok := s.ident[ident]
	if !ok {
		return nil
	}
	v, ok := i[seq]
	if ok {
		delete(i, seq)
	}
	return v
}
