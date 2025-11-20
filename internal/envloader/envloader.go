package envloader

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

// Load reads environment variables from the provided files (default ".env").
// Missing files are ignored, other errors are logged.
func Load(files ...string) {
	names := files
	if len(names) == 0 {
		names = []string{".env"}
	}

	for _, name := range names {
		if _, err := os.Stat(name); err != nil {
			if !os.IsNotExist(err) {
				log.Printf("[ENV] unable to access %s: %v", name, err)
			}
			continue
		}
		if err := godotenv.Load(name); err != nil {
			log.Printf("[ENV] failed to load %s: %v", name, err)
			continue
		}
		log.Printf("[ENV] loaded %s", name)
	}
}
