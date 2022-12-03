package chat

type GetIdFunc func(any) any

type Repository[C any] struct {
	items []*C
	getId GetIdFunc
}

func NewRepository[C any](getIdFunc GetIdFunc) *Repository[C] {
	return &Repository[C]{getId: getIdFunc}
}

func (r *Repository[C]) Add(item *C) bool {
	if _, ok := r.pos(item); !ok {
		r.items = append(r.items, item)
		return true
	}
	return false
}

func (r *Repository[C]) Update(item *C) bool {
	if i, ok := r.pos(item); ok {
		r.items[i] = item
		return true
	}
	return false
}

func (r *Repository[C]) Remove(id any) bool {
	if item, ok := r.Get(id); ok {
		if i, ok := r.pos(item); ok {
			r.items = append(r.items[:i], r.items[i+1:]...)
			return true
		}
	}
	return false
}

func (r *Repository[C]) Get(id any) (*C, bool) {
	filtered := r.filter(func(c *C) bool {
		return r.getId(c) == id
	})

	if len(filtered) > 0 {
		return filtered[0], true
	}
	return nil, false
}

func (r *Repository[C]) GetAll() []*C {
	return r.items
}

func (r *Repository[C]) Filter(f func(*C) bool) []*C {
	return r.filter(f)
}

func (r *Repository[C]) pos(value *C) (int, bool) {
	for p, v := range r.items {
		if r.getId(v) == r.getId(value) {
			return p, true
		}
	}
	return -1, false
}

func (r *Repository[C]) filter(f func(*C) bool) []*C {
	fltd := make([]*C, 0)
	for _, e := range r.items {

		if f(e) {
			fltd = append(fltd, e)
		}
	}
	return fltd
}
