package hclconfig

import (
	"errors"
	"fmt"

	"github.com/hashicorp/hcl/v2"
)

func GetTraversalAttr(t hcl.Traversal, expectedRootName string, index int) (hcl.TraverseAttr, error) {
	if index == 0 {
		return hcl.TraverseAttr{}, errors.New("index 0 is TraerseRoot")
	}
	if index > len(t)-1 {
		return hcl.TraverseAttr{}, fmt.Errorf("can not access index %d, traversal length is %d", index, len(t))
	}
	if t.IsRelative() {
		return hcl.TraverseAttr{}, errors.New("traversal is relative")
	}
	if t.RootName() != expectedRootName {
		return hcl.TraverseAttr{}, fmt.Errorf("expected root name is %s, actual %s", expectedRootName, t.RootName())
	}
	attr, ok := t[index].(hcl.TraverseAttr)
	if !ok {
		return hcl.TraverseAttr{}, fmt.Errorf("traversal[%d] is not TraverseAttr", index)
	}
	return attr, nil
}
