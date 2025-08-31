package protoreg

import (
	"hash/fnv"
	"sort"

	"github.com/jhump/protoreflect/v2/protobuilder"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func allocateFieldNumbers(fieldBuilders []*protobuilder.FieldBuilder) {
	fieldNames := make([]string, len(fieldBuilders))
	for i, fb := range fieldBuilders {
		fieldNames[i] = string(fb.Name())
	}
	fieldNumbers := getFnv32LP(fieldNames)
	for i, fb := range fieldBuilders {
		fb.SetNumber(protoreflect.FieldNumber(fieldNumbers[i]))
	}
}

func allocateEnumValueNumbers(enumValueBuilders []*protobuilder.EnumValueBuilder) {
	valueNames := make([]string, len(enumValueBuilders))
	for i, evb := range enumValueBuilders {
		valueNames[i] = string(evb.Name())
	}
	valueNumbers := getFnv32LP(valueNames)
	for i, evb := range enumValueBuilders {
		evb.SetNumber(protoreflect.EnumNumber(valueNumbers[i]))
	}
}

// getFnv32LP assigns deterministic proto tag numbers following the specification:
// 1. candidate = (FNV32a(name) % 31767) + 1 (range 1..31767)
// 2. if candidate in [19000,19999] -> linear probe (candidate+1 wrapping to 1)
// 3. if collision -> linear probe (skip reserved block) until free
// Order: we sort names to ensure stable collision resolution, then map back to original indices.
func getFnv32LP(names []string) []int {
	if len(names) == 0 {
		return nil
	}
	type item struct {
		name string
		idx  int
	}
	items := make([]item, len(names))
	for i, n := range names {
		items[i] = item{name: n, idx: i}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].name < items[j].name })

	out := make([]int, len(names))
	used := make(map[int]struct{}, len(names))
	const max = 31767
	for _, it := range items {
		start := int(fnv32(it.name)%31767) + 1 // 1..31767
		cand := start
		for {
			if cand >= 19000 && cand <= 19999 { // reserved block skip
				cand = 20000
				if cand > max { // wrap after reserved block overflow
					cand = 1
				}
				if cand == start { // rare pathological full cycle
					panic("allocateFieldNumbers: exhausted tag space (reserved block)")
				}
				continue
			}
			if _, ok := used[cand]; !ok {
				used[cand] = struct{}{}
				out[it.idx] = cand
				break
			}
			cand++
			if cand > max {
				cand = 1
			}
			if cand == start { // full cycle
				panic("allocateFieldNumbers: exhausted tag space")
			}
		}
	}
	return out
}

func fnv32(s string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(s))
	return h.Sum32()
}
