package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
)

var (
	ErrEmptyTitle       = errors.New("title cannot be empty")
	ErrTitleTooLong     = errors.New("title is too long")
	ErrPricePositive    = errors.New("price must be positive")
	ErrStockNonNegative = errors.New("stock must be non-negative")
	ErrQtyPositive      = errors.New("quantity must be positive")
	ErrUnknownCommand   = errors.New("unknown command")
	ErrInvalidArguments = errors.New("invalid arguments")
)

func main() {
	storageType := flag.String("storage", "memory", "storage type: memory or json")
	filePath := flag.String("file", "", "path to json file")
	flag.Parse()

	var store Storage
	if *storageType == "json" {
		if *filePath == "" {
			fmt.Fprintln(os.Stderr, "Error: --file is required when --storage=json")
			os.Exit(1)
		}
		var err error
		js, err := NewJSONStorage(*filePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to load storage: %v\n", err)
			os.Exit(1)
		}
		store = js
	} else {
		store = NewMemoryStorage()
	}

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		if err := execute(line, store); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: reading stdin: %v\n", err)
	}
}

func execute(line string, store Storage) error {
	parts := strings.SplitN(line, " ", 2)
	cmd := parts[0]
	args := ""
	if len(parts) > 1 {
		args = strings.TrimSpace(parts[1])
	}

	switch cmd {
	case "add":
		return handleAdd(args, store)
	case "list":
		if args != "" {
			return ErrInvalidArguments
		}
		return handleList(store)
	case "buy":
		return handleBuy(args, store)
	case "delete":
		return handleDelete(args, store)
	case "exit":
		fmt.Println("Goodbye!")
		os.Exit(0)
	default:
		return fmt.Errorf("%w '%s'", ErrUnknownCommand, cmd)
	}
	return nil
}

func handleAdd(args string, store Storage) error {
	// Формат: "Title" price stock
	if !strings.HasPrefix(args, "\"") {
		return ErrInvalidArguments
	}
	rest := args[1:]
	endQuote := strings.Index(rest, "\"")
	if endQuote == -1 {
		return ErrInvalidArguments
	}
	title := rest[:endQuote]
	rest = strings.TrimSpace(rest[endQuote+1:])

	fields := strings.Fields(rest)
	if len(fields) != 2 {
		return ErrInvalidArguments
	}

	price, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return ErrInvalidArguments
	}
	stock, err := strconv.Atoi(fields[1])
	if err != nil {
		return ErrInvalidArguments
	}

	if title == "" {
		return ErrEmptyTitle
	}
	if len(title) > 200 {
		return ErrTitleTooLong
	}
	if price <= 0 {
		return ErrPricePositive
	}
	if stock < 0 {
		return ErrStockNonNegative
	}

	id, err := store.Add(title, price, stock)
	if err != nil {
		return err
	}
	fmt.Printf("Listing added with ID: %d\n", id)
	return nil
}

func handleList(store Storage) error {
	listings := store.List()
	for _, l := range listings {
		fmt.Printf("%d. \"%s\" - %.2f (stock: %d)\n", l.ID, l.Title, l.Price, l.Stock)
	}
	return nil
}

func handleBuy(args string, store Storage) error {
	fields := strings.Fields(args)
	if len(fields) != 2 {
		return ErrInvalidArguments
	}
	id, err := strconv.Atoi(fields[0])
	if err != nil {
		return ErrInvalidArguments
	}
	qty, err := strconv.Atoi(fields[1])
	if err != nil {
		return ErrInvalidArguments
	}

	if qty <= 0 {
		return ErrQtyPositive
	}

	remaining, err := store.Buy(id, qty)
	if err != nil {
		return err
	}
	fmt.Printf("Bought %d of listing %d (remaining: %d)\n", qty, id, remaining)
	return nil
}

func handleDelete(args string, store Storage) error {
	fields := strings.Fields(args)
	if len(fields) != 1 {
		return ErrInvalidArguments
	}
	id, err := strconv.Atoi(fields[0])
	if err != nil {
		return ErrInvalidArguments
	}

	if err := store.Delete(id); err != nil {
		return err
	}
	fmt.Printf("Listing %d deleted\n", id)
	return nil
}
