package ping

type EntryHeap []*entry


func (eh *EntryHeap) Push(x interface{}) {
	n := len(*eh)
	item := x.(*entry)
	item.index = n
	*eh = append(*eh, item)
}


func (eh *EntryHeap) Peek() interface{} {
	old := *eh
	n := len(old)
	if n <= 0 {
		return nil
	}
	return old[0]
}


func (eh *EntryHeap) Pop() interface{} {
	old := *eh
	n := len(old)
	item := old[n-1]
	item.index = -1 // for safety
	*eh = old[0 : n-1]
	return item
}


func (eh *EntryHeap) Len() int { return len(*eh) }

func (eh *EntryHeap) Less(i, j int) bool {
	return (*eh)[i].evTime.Before((*eh)[j].evTime)
}

func (eh *EntryHeap) Swap(i, j int) {
	(*eh)[i], (*eh)[j] = (*eh)[j], (*eh)[i]
	(*eh)[i].index = i
	(*eh)[j].index = j
}