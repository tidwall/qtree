package qtree

import (
	"github.com/tidwall/geoindex"
	"github.com/tidwall/geoindex/child"
)

const maxEntries = 32
const minEntries = maxEntries * 40 / 100
const maxLevel = 16

// QTree ...
type QTree struct {
	init bool
	min  [2]float64
	max  [2]float64
	root node
}

// New creates a new QuadTree
func New(min, max [2]float64) *QTree {
	return &QTree{min: min, max: max, init: true}
}

var _ geoindex.Interface = &QTree{}

// Insert an item into the structure
func (tr *QTree) Insert(min [2]float64, max [2]float64, data interface{}) {
	if !tr.init {
		*tr = *New([2]float64{-180, -90}, [2]float64{180, 90})
	}
	tr.root.insert(tr.min, tr.max, 1, entry{min, max, data})
}

type entry struct {
	min, max [2]float64
	data     interface{}
}

type node struct {
	count   int
	entries []entry
	quads   *[4]node
}

func (n *node) insert(min, max [2]float64, level int, e entry) {
	if n.quads == nil {
		if level != maxLevel && len(n.entries) == maxEntries {
			n.split(min, max, level)
			n.insert(min, max, level, e)
			return
		}
		n.entries = append(n.entries, e)
	} else {
		qmin, qmax, quad := choose(min, max, e)
		if quad == -1 {
			n.entries = append(n.entries, e)
		} else {
			n.quads[quad].insert(qmin, qmax, level+1, e)
		}
	}
	n.count++
}

func (n *node) split(min, max [2]float64, level int) {
	entries := n.entries
	n.quads = new([4]node)
	n.entries = nil
	n.count = 0
	for _, e := range entries {
		n.insert(min, max, level, e)
	}
}

func (n *node) join() {
	n.entries = append(n.entries, n.quads[0].entries...)
	n.entries = append(n.entries, n.quads[1].entries...)
	n.entries = append(n.entries, n.quads[2].entries...)
	n.entries = append(n.entries, n.quads[3].entries...)
	n.quads = nil
}

func (n *node) delete(min, max [2]float64, e entry) bool {
	for i := 0; i < len(n.entries); i++ {
		if n.entries[i].data == e.data {
			n.entries[i] = n.entries[len(n.entries)-1]
			n.entries[len(n.entries)-1] = entry{}
			n.entries = n.entries[:len(n.entries)-1]
			n.count--
			return true
		}
	}
	if n.quads != nil {
		qmin, qmax, quad := choose(min, max, e)
		if quad != -1 {
			if n.quads[quad].delete(qmin, qmax, e) {
				n.count--
				if n.count < minEntries {
					n.join()
				}
				return true
			}
		}
	}
	return false
}

func (n *node) search(
	min, max [2]float64, tmin, tmax [2]float64,
	iter func(min, max [2]float64, data interface{}) bool,
) bool {
	for _, e := range n.entries {
		if intersects(tmin, tmax, e.min, e.max) {
			if !iter(e.min, e.max, e.data) {
				return false
			}
		}
	}
	if n.quads != nil {
		if n.quads[0].count > 0 {
			if qmin, qmax := q0(min, max); intersects(tmin, tmax, qmin, qmax) {
				if !n.quads[0].search(qmin, qmax, tmin, tmax, iter) {
					return false
				}
			}
		}
		if n.quads[1].count > 0 {
			if qmin, qmax := q1(min, max); intersects(tmin, tmax, qmin, qmax) {
				if !n.quads[1].search(qmin, qmax, tmin, tmax, iter) {
					return false
				}
			}
		}
		if n.quads[2].count > 0 {
			if qmin, qmax := q2(min, max); intersects(tmin, tmax, qmin, qmax) {
				if !n.quads[2].search(qmin, qmax, tmin, tmax, iter) {
					return false
				}
			}
		}
		if n.quads[3].count > 0 {
			if qmin, qmax := q3(min, max); intersects(tmin, tmax, qmin, qmax) {
				if !n.quads[3].search(qmin, qmax, tmin, tmax, iter) {
					return false
				}
			}
		}
	}
	return true
}

func (n *node) scan(
	iter func(min, max [2]float64, data interface{}) bool,
) bool {
	for _, e := range n.entries {
		if !iter(e.min, e.max, e.data) {
			return false
		}
	}
	if n.quads != nil {
		if !n.quads[0].scan(iter) {
			return false
		}
		if !n.quads[1].scan(iter) {
			return false
		}
		if !n.quads[2].scan(iter) {
			return false
		}
		if !n.quads[3].scan(iter) {
			return false
		}
	}
	return true
}

// Delete an item from the structure
func (tr *QTree) Delete(min, max [2]float64, data interface{}) {
	if !tr.init {
		return
	}
	tr.root.delete(tr.min, tr.max, entry{min, max, data})
}

// Replace an item in the structure. This is effectively just a Delete
// followed by an Insert. But for some structures it may be possible to
// optimize the operation to avoid multiple passes
func (tr *QTree) Replace(
	oldMin, oldMax [2]float64, oldData interface{},
	newMin, newMax [2]float64, newData interface{},
) {
	tr.Delete(oldMin, oldMax, oldData)
	tr.Insert(newMin, newMax, newData)
}

// Search the structure for items that intersects the rect param
func (tr *QTree) Search(
	min, max [2]float64,
	iter func(min, max [2]float64, data interface{}) bool,
) {
	if !tr.init {
		return
	}
	tr.root.search(tr.min, tr.max, min, max, iter)
}

// Scan iterates through all data in tree in no specified order.
func (tr *QTree) Scan(iter func(min, max [2]float64, data interface{}) bool) {
	tr.root.scan(iter)
}

// Len returns the number of items in tree
func (tr *QTree) Len() int {
	return tr.root.count
}

// Bounds returns the minimum bounding box
func (tr *QTree) Bounds() (min, max [2]float64) {
	if !tr.init {
		return min, max
	}
	minX, _ := tr.root.min(tr.min, tr.max, 0, 0, false)
	minY, _ := tr.root.min(tr.min, tr.max, 1, 0, false)
	maxX, _ := tr.root.max(tr.min, tr.max, 0, 0, false)
	maxY, _ := tr.root.max(tr.min, tr.max, 1, 0, false)
	return [2]float64{minX, minY}, [2]float64{maxX, maxY}
}

type rnode struct {
	min, max [2]float64
	node     *node
}

// Children returns all children for parent node. If parent node is nil
// then the root nodes should be returned.
// The reuse buffer is an empty length slice that can optionally be used
// to avoid extra allocations.
func (tr *QTree) Children(parent interface{}, reuse []child.Child) (
	children []child.Child,
) {
	children = reuse[:0]
	if parent == nil {
		if tr.Len() > 0 {
			// fill with the root
			children = append(children, child.Child{
				Min:  tr.min,
				Max:  tr.max,
				Data: &rnode{tr.min, tr.max, &tr.root},
				Item: false,
			})
		}
	} else {
		// fill with child items
		switch n := parent.(type) {
		case *rnode:
			if n.node.quads == nil {
				// leaf
				for _, e := range n.node.entries {
					children = append(children, child.Child{
						Min:  e.min,
						Max:  e.max,
						Data: e.data,
						Item: true,
					})
				}
				return children
			}

			if len(n.node.entries) > 0 {
				children = append(children, child.Child{
					Min:  n.min,
					Max:  n.max,
					Data: n.node.entries,
					Item: false,
				})
			}
			if n.node.quads[0].count > 0 {
				qmin, qmax := q0(n.min, n.max)
				children = append(children, child.Child{
					Min:  qmin,
					Max:  qmax,
					Data: &rnode{qmin, qmax, &n.node.quads[0]},
					Item: false,
				})
			}
			if n.node.quads[1].count > 0 {
				qmin, qmax := q1(n.min, n.max)
				children = append(children, child.Child{
					Min:  qmin,
					Max:  qmax,
					Data: &rnode{qmin, qmax, &n.node.quads[1]},
					Item: false,
				})
			}
			if n.node.quads[2].count > 0 {
				qmin, qmax := q2(n.min, n.max)
				children = append(children, child.Child{
					Min:  qmin,
					Max:  qmax,
					Data: &rnode{qmin, qmax, &n.node.quads[2]},
					Item: false,
				})
			}
			if n.node.quads[3].count > 0 {
				qmin, qmax := q3(n.min, n.max)
				children = append(children, child.Child{
					Min:  qmin,
					Max:  qmax,
					Data: &rnode{qmin, qmax, &n.node.quads[3]},
					Item: false,
				})
			}
		case []entry:
			for _, e := range n {
				children = append(children, child.Child{
					Min:  e.min,
					Max:  e.max,
					Data: e.data,
					Item: true,
				})
			}
		}
	}
	return children
}

func intersects(amin, amax, bmin, bmax [2]float64) bool {
	if bmin[0] > amax[0] || bmax[0] < amin[0] {
		return false
	}
	if bmin[1] > amax[1] || bmax[1] < amin[1] {
		return false
	}
	return true
}

func contains(amin, amax, bmin, bmax [2]float64) bool {
	return bmin[0] >= amin[0] && bmax[0] <= amax[0] &&
		bmin[1] >= amin[1] && bmax[1] <= amax[1]
}

func q0min(min, max [2]float64) [2]float64 {
	return [2]float64{min[0], min[1]}
}
func q0max(min, max [2]float64) [2]float64 {
	return [2]float64{(max[0] + min[0]) / 2, (max[1] + min[1]) / 2}
}
func q0(min, max [2]float64) ([2]float64, [2]float64) {
	return q0min(min, max), q0max(min, max)
}

func q1min(min, max [2]float64) [2]float64 {
	return [2]float64{(max[0] + min[0]) / 2, min[1]}
}
func q1max(min, max [2]float64) [2]float64 {
	return [2]float64{max[0], (max[1] + min[1]) / 2}
}
func q1(min, max [2]float64) ([2]float64, [2]float64) {
	return q1min(min, max), q1max(min, max)
}

func q2min(min, max [2]float64) [2]float64 {
	return [2]float64{min[0], (max[1] + min[1]) / 2}
}
func q2max(min, max [2]float64) [2]float64 {
	return [2]float64{(max[0] + min[0]) / 2, max[1]}
}
func q2(min, max [2]float64) ([2]float64, [2]float64) {
	return q2min(min, max), q2max(min, max)
}

func q3min(min, max [2]float64) [2]float64 {
	return [2]float64{(max[0] + min[0]) / 2, (max[1] + min[1]) / 2}
}
func q3max(min, max [2]float64) [2]float64 {
	return [2]float64{max[0], max[1]}
}
func q3(min, max [2]float64) ([2]float64, [2]float64) {
	return q3min(min, max), q3max(min, max)
}

func choose(min, max [2]float64, e entry) (qmin, qmax [2]float64, quad int) {
	if qmin, qmax = q0(min, max); contains(qmin, qmax, e.min, e.max) {
		return qmin, qmax, 0
	}
	if qmin, qmax = q1(min, max); contains(qmin, qmax, e.min, e.max) {
		return qmin, qmax, 1
	}
	if qmin, qmax = q2(min, max); contains(qmin, qmax, e.min, e.max) {
		return qmin, qmax, 2
	}
	if qmin, qmax = q3(min, max); contains(qmin, qmax, e.min, e.max) {
		return qmin, qmax, 3
	}
	return qmin, qmax, -1
}

func (n *node) min(min, max [2]float64, axis int, v float64, set bool) (
	float64, bool,
) {
	for _, e := range n.entries {
		if !set || e.min[axis] < v {
			v, set = e.min[axis], true
		}
	}
	if (set && v < min[axis]) || n.quads == nil {
		return v, set
	}
	if axis == 0 {
		qmin, qmax := q0(min, max)
		qv, qset := n.quads[0].min(qmin, qmax, axis, v, set)
		if qset && (!set || qv < v) {
			v, set = qv, true
		}
		qmin, qmax = q2(min, max)
		qv, qset = n.quads[2].min(qmin, qmax, axis, v, set)
		if qset && (!set || qv < v) {
			v, set = qv, true
		}
	}
	return v, set
}

func (n *node) max(min, max [2]float64, axis int, v float64, set bool) (
	float64, bool,
) {
	for _, e := range n.entries {
		if !set || e.max[axis] > v {
			v, set = e.max[axis], true
		}
	}
	if (set && v > max[axis]) || n.quads == nil {
		return v, set
	}
	if axis == 0 {
		qmin, qmax := q0(min, max)
		qv, qset := n.quads[0].max(qmin, qmax, axis, v, set)
		if qset && (!set || qv > v) {
			v, set = qv, true
		}
		qmin, qmax = q2(min, max)
		qv, qset = n.quads[2].max(qmin, qmax, axis, v, set)
		if qset && (!set || qv > v) {
			v, set = qv, true
		}
	}
	return v, set
}
