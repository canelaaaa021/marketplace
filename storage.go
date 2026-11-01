package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

var (
	ErrListingNotFound   = errors.New("listing not found")
	ErrInsufficientStock = errors.New("insufficient stock")
)

type Listing struct {
	ID    int     `json:"id"`
	Title string  `json:"title"`
	Price float64 `json:"price"`
	Stock int     `json:"stock"`
}

type state struct {
	NextID   int       `json:"next_id"`
	Listings []Listing `json:"listings"`
}

type Storage interface {
	Add(title string, price float64, stock int) (int, error)
	List() []Listing
	Buy(id int, qty int) (int, error)
	Delete(id int) error
}

// MemoryStorage реализует in-memory хранилище
type MemoryStorage struct {
	mu       sync.RWMutex
	nextID   int
	listings map[int]Listing
}

func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		nextID:   1,
		listings: make(map[int]Listing),
	}
}

func (s *MemoryStorage) Add(title string, price float64, stock int) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := s.nextID
	s.nextID++

	s.listings[id] = Listing{ID: id, Title: title, Price: price, Stock: stock}
	return id, nil
}

func (s *MemoryStorage) List() []Listing {
	s.mu.RLock()
	defer s.mu.RUnlock()

	res := make([]Listing, 0, len(s.listings))
	for _, l := range s.listings {
		res = append(res, l)
	}
	sort.Slice(res, func(i, j int) bool { return res[i].ID < res[j].ID })
	return res
}

func (s *MemoryStorage) Buy(id int, qty int) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	l, ok := s.listings[id]
	if !ok {
		return 0, ErrListingNotFound
	}
	if l.Stock < qty {
		return 0, ErrInsufficientStock
	}

	l.Stock -= qty
	s.listings[id] = l
	return l.Stock, nil
}

func (s *MemoryStorage) Delete(id int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.listings[id]; !ok {
		return ErrListingNotFound
	}
	delete(s.listings, id)
	return nil
}

// JSONStorage оборачивает MemoryStorage и сохраняет данные в файл
type JSONStorage struct {
	*MemoryStorage
	filePath string
}

func NewJSONStorage(filePath string) (*JSONStorage, error) {
	mem := NewMemoryStorage()
	js := &JSONStorage{
		MemoryStorage: mem,
		filePath:      filePath,
	}

	if err := js.load(); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	return js, nil
}

func (js *JSONStorage) load() error {
	data, err := os.ReadFile(js.filePath)
	if err != nil {
		return err
	}

	var st state
	if err := json.Unmarshal(data, &st); err != nil {
		return err // Если JSON битый - падаем (по ТЗ валидный JSON)
	}

	js.nextID = st.NextID
	if js.nextID == 0 {
		js.nextID = 1 // fallback
	}
	for _, l := range st.Listings {
		js.listings[l.ID] = l
	}
	return nil
}

func (js *JSONStorage) save() error {
	js.mu.RLock()
	st := state{
		NextID:   js.nextID,
		Listings: make([]Listing, 0, len(js.listings)),
	}
	for _, l := range js.listings {
		st.Listings = append(st.Listings, l)
	}
	// Сортируем для стабильного JSON
	sort.Slice(st.Listings, func(i, j int) bool { return st.Listings[i].ID < st.Listings[j].ID })
	js.mu.RUnlock()

	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}

	dir := filepath.Dir(js.filePath)
	tmp, err := os.CreateTemp(dir, filepath.Base(js.filePath)+".tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // Уберем за собой, если что-то пойдет не так

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	return os.Rename(tmpName, js.filePath)
}

func (js *JSONStorage) Add(title string, price float64, stock int) (int, error) {
	id, err := js.MemoryStorage.Add(title, price, stock)
	if err != nil {
		return 0, err
	}
	return id, js.save()
}

func (js *JSONStorage) Buy(id int, qty int) (int, error) {
	remaining, err := js.MemoryStorage.Buy(id, qty)
	if err != nil {
		return 0, err
	}
	return remaining, js.save()
}

func (js *JSONStorage) Delete(id int) error {
	err := js.MemoryStorage.Delete(id)
	if err != nil {
		return err
	}
	return js.save()
}
