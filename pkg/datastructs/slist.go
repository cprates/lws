package datastructs

// Node represents a container that SList is composed of.
type Node struct {
	Data interface{}
	next *Node
}

// SList represents a Simply Linked List.
type SList struct {
	Head *Node
	Tail *Node
	Size int
}

// NewSList returns a new ready to use simply linked list.
func NewSList() *SList {
	return &SList{}
}

// Next returns the next node of the list or nil is none.
func (n *Node) Next() *Node {
	return n.next
}

// Append appends the element to the end of the list.
func (s *SList) Append(data interface{}) *SList {

	s.Size++
	newNode := &Node{Data: data}

	if s.Head == nil {
		s.Head = newNode
		s.Tail = newNode

		return s
	}

	s.Tail.next = newNode
	s.Tail = newNode

	return s
}

// AppendN appends n nodes to the end of the list.
func (s *SList) AppendN(nodes []*Node) *SList {

	var first *Node
	var current *Node
	for _, node := range nodes {
		// if someone is recycling the node, make sure it's 'clean'
		node.next = nil
		s.Size++
		if first == nil {
			first = node
			current = node
			continue
		}
		current.next = node
		current = node
	}

	if s.Head == nil {
		s.Head = first
		s.Tail = current

		return s
	}

	s.Tail.next = first
	s.Tail = current

	return s
}

// Take takes the first element of the list, or return ErrEmptyList if the list if empty.
func (s *SList) Take() (node *Node, err error) {

	if s.Size == 0 {
		err = ErrEmptyList
		return
	}

	s.Size--
	node = s.Head

	s.Head = s.Head.next
	if s.Head == nil {
		s.Tail = nil
		return
	}

	return
}

// TakeN takes n nodes from the front of the list.
func (s *SList) TakeN(n int) (nodes []*Node, err error) {

	if n == 0 {
		return
	}
	if s.Size < n {
		err = ErrNoEnoughElems
		return
	}

	s.Size -= n
	last := s.Head
	for i := 0; i < n; i++ {
		nodes = append(nodes, last)
		last = last.next
	}

	s.Head = last
	if last == nil {
		s.Tail = nil
	}
	return
}

// TakeUpToN is the same as takeN but don't return error if the list does not contain n elements,
// it returns n or less nodes, depending on what the list contains.
func (s *SList) TakeUpToN(n int) (nodes []*Node) {

	if n == 0 || s.Size == 0 {
		return
	}

	last := s.Head

	for i := 0; i < n && last != nil; i++ {
		s.Size--
		nodes = append(nodes, last)
		last = last.next
	}

	s.Head = last
	if last == nil {
		s.Tail = nil
	}
	return
}
