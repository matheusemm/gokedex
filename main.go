package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/matheusemm/gokedex/internal/pokecache"
)

const PROMPT = "Pokedex > "

const (
	POKE_API_BASE_URL   = "https://pokeapi.co/api/v2"
	POKEMON_PATH        = "/pokemon"
	LOCATION_AREAS_PATH = "/location-area"

	PAGE_SIZE = 20
)

func main() {
	cfg := newConfig()

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print(PROMPT)

		ok := scanner.Scan()
		if !ok {
			if scanner.Err() != nil {
				// this is an error
				fmt.Fprintf(os.Stderr, "failed to scan: %v\n", scanner.Err())
			} else {
				fmt.Println("EOF")
			}
		}

		fields := strings.Fields(strings.ToLower(scanner.Text()))
		action := ""
		args := []string{}

		if len(fields) > 0 {
			action = fields[0]
			args = fields[1:]
		}

		cmd, ok := cfg.commands[action]
		if !ok {
			fmt.Println("Unknown command")
			continue
		}

		if err := cmd.callback(cfg, args...); err != nil {
			fmt.Fprintf(os.Stderr, "failed to execute command %q: %v\n", cmd.name, err)
		}
	}
}

type config struct {
	commands map[string]cliCommand
	next     *string
	previous *string
	cache    *pokecache.Cache
	pokemons map[string]Pokemon
}

func newConfig() *config {
	commands := map[string]cliCommand{
		"exit": {
			name:        "exit",
			description: "Exits the Pokedex",
			callback:    commandExit,
		},
		"help": {
			name:        "help",
			description: "Displays a help message",
			callback:    commandHelp,
		},
		"map": {
			name:        "map",
			description: fmt.Sprintf("Displays the next %d location areas", PAGE_SIZE),
			callback:    commandMap,
		},
		"mapb": {
			name:        "mapb",
			description: fmt.Sprintf("Displays the previous %d location areas", PAGE_SIZE),
			callback:    commandMapb,
		},
		"explore": {
			name:        "explore",
			description: "Explores the named area and returns a list of all Pokémons located there",
			callback:    commandExplore,
		},
		"catch": {
			name:        "catch",
			description: "Tries to catch the named Pokémon",
			callback:    commandCatch,
		},
		"inspect": {
			name:        "inspect",
			description: "Inspects the named Pokémon if was already captured",
			callback:    commandInspect,
		},
	}

	u, err := buildURL(LOCATION_AREAS_PATH, 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to build URL: %v\n", err)
		os.Exit(1)
	}

	return &config{
		commands: commands,
		next:     &u,
		previous: nil,
		cache:    pokecache.NewCache(10 * time.Second),
		pokemons: make(map[string]Pokemon),
	}
}

type cliCommand struct {
	name        string
	description string
	callback    func(cfg *config, args ...string) error
}

func commandExit(_ *config, _ ...string) error {
	fmt.Println("Closing the Pokedex... Goodbye!")
	os.Exit(0)

	return nil
}

func commandHelp(cfg *config, _ ...string) error {
	fmt.Println("Welcome to the Pokedex!")
	fmt.Print("Usage:\n\n")
	for name, cmd := range cfg.commands {
		fmt.Printf("%s: %s\n", name, cmd.description)
	}

	return nil
}

func commandMap(cfg *config, _ ...string) error {
	if cfg.next == nil {
		fmt.Println("You are on the last page")
		return nil
	}

	var getLocationAreas GetLocationAreas
	if err := cacheOrGet(cfg, *cfg.next, &getLocationAreas); err != nil {
		return err
	}

	for _, area := range getLocationAreas.Results {
		fmt.Println(area.Name)
	}

	cfg.next = getLocationAreas.Next
	cfg.previous = getLocationAreas.Previous

	return nil
}

func commandMapb(cfg *config, _ ...string) error {
	if cfg.previous == nil {
		fmt.Println("You are on the first page")
		return nil
	}

	var getLocationAreas GetLocationAreas
	if err := cacheOrGet(cfg, *cfg.previous, &getLocationAreas); err != nil {
		return err
	}

	for _, area := range getLocationAreas.Results {
		fmt.Println(area.Name)
	}

	cfg.next = getLocationAreas.Next
	cfg.previous = getLocationAreas.Previous

	return nil
}

func commandExplore(cfg *config, args ...string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing area name")
	}

	u := fmt.Sprintf("%s/%s/%s", POKE_API_BASE_URL, LOCATION_AREAS_PATH, args[0])

	var getLocationArea GetLocationArea
	if err := cacheOrGet(cfg, u, &getLocationArea); err != nil {
		return err
	}

	fmt.Printf("Exploring %s...\n", args[0])
	fmt.Println("Found Pokémon:")
	for _, encounter := range getLocationArea.PokemonEncounters {
		fmt.Println(" -", encounter.Pokemon.Name)
	}

	return nil
}

func commandCatch(cfg *config, args ...string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing Pokémon name")
	}

	u := fmt.Sprintf("%s/%s/%s", POKE_API_BASE_URL, POKEMON_PATH, args[0])

	var pokemon Pokemon
	if err := cacheOrGet(cfg, u, &pokemon); err != nil {
		return err
	}

	// The max experience value found at https://bulbapedia.bulbagarden.net/wiki/List_of_Pok%C3%A9mon_by_effort_value_yield_in_Generation_IX
	// is for Blissey: 635. I chose to set `maxBaseExperience` to 700 to make it
	// possible to catch Blissey when throwing a Pokeball.
	maxBaseExperience := 700
	baseExperience := pokemon.BaseExperience
	threshold := float64(baseExperience) / float64(maxBaseExperience)

	fmt.Printf("Throwing a Pokeball at %s...\n", args[0])

	roll := rand.Float64()
	if roll > threshold {
		cfg.pokemons[pokemon.Name] = pokemon
		fmt.Printf("%s was caught!\n", pokemon.Name)
	} else {
		fmt.Printf("%s escaped!\n", pokemon.Name)
	}

	return nil
}

func commandInspect(cfg *config, args ...string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing Pokémon name")
	}

	pokemon, ok := cfg.pokemons[args[0]]
	if !ok {
		fmt.Println("you have not caught that Pokémon")
		return nil
	}

	fmt.Printf("Name: %s\n", pokemon.Name)
	fmt.Printf("Height: %d\n", pokemon.Height)
	fmt.Printf("Weight: %d\n", pokemon.Weight)

	fmt.Println("Stats:")
	for _, stat := range pokemon.Stats {
		fmt.Printf("  - %s: %d\n", stat.Stat.Name, stat.BaseStat)
	}

	fmt.Println("Types:")
	for _, typ := range pokemon.Types {
		fmt.Printf("  - %s\n", typ.Type.Name)
	}

	return nil
}

func buildURL(path string, offset int) (string, error) {
	u, err := url.Parse(POKE_API_BASE_URL + "/" + path)
	if err != nil {
		return "", err
	}

	q := u.Query()
	q.Set("limit", "20")
	q.Set("offset", strconv.Itoa(offset))

	u.RawQuery = q.Encode()
	return u.String(), nil
}

func cacheOrGet(cfg *config, u string, v any) error {
	var content []byte
	if cached, ok := cfg.cache.Get(u); ok {
		content = cached
	} else {
		res, err := http.Get(u)
		if err != nil {
			return fmt.Errorf("failed to GET %s: %w", u, err)
		}
		defer res.Body.Close()

		bytes, err := io.ReadAll(res.Body)
		if err != nil {
			return fmt.Errorf("failed to read response body: %w", err)
		}
		content = bytes

		cfg.cache.Add(u, content)
	}

	if err := json.Unmarshal(content, v); err != nil {
		return fmt.Errorf("failed to decode JSON: %w", err)
	}
	return nil
}
