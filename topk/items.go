package topk

type item struct {
	value     int32
	frequency int
	index     int
}

// itemList is just a sorted item list by frequency.
// after changing the frequency, you must call fix with the item in order to fix the ordering.
type itemList []*item

func (il *itemList) put(i *item) {
	orig := *il
	i.index = len(orig)
	*il = append(orig, i)
	il.fix(i)
}

func (il *itemList) remove(i *item) {
	idx := i.index
	l := *il

	if i.frequency == l[len(l)-1].frequency {
		il.swap(idx, len(l)-1)
		idx = i.index
	}

	if idx < len(l)-1 {
		copy(l[idx:], l[idx+1:])
	}
	*il = l[0 : len(l)-1]
}

func (il itemList) fix(i *item) {
	if len(il) < 2 {
		return
	}

	idx := i.index
	for idx < len(il)-1 && il[idx+1].frequency > i.frequency {
		il.swap(idx, idx+1)
		idx = i.index
	}

	for idx > 0 && il[idx-1].frequency < i.frequency {
		il.swap(idx, idx-1)
		idx = i.index
	}
}

func (il itemList) swap(i, j int) {
	il[i], il[j] = il[j], il[i]
	il[i].index = i
	il[j].index = j
}
