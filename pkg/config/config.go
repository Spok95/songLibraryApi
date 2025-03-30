package config

import (
	"fmt"
	"github.com/joho/godotenv"
	"log"
	"os"
)

// Config хранит параметры конфигурации
type Config struct {
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	DBSSLMode  string
	MockAPIURL string
}

// LoadConfig загружает переменные окружения
func LoadConfig() (*Config, error) {
	// Загружаем .env, если он есть
	if err := godotenv.Load(); err != nil {
		log.Println("⚠️ .env файл не найден, используем переменные окружения")
	}

	// Читаем переменные
	cfg := &Config{
		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBUser:     getEnv("DB_USER", "postgres"),
		DBPassword: getEnv("DB_PASSWORD", "password"),
		DBName:     getEnv("DB_NAME", "songlibrary"),
		DBSSLMode:  getEnv("DB_SSLMODE", "disable"),
		MockAPIURL: getEnv("MOCK_API_URL", "http://mockapi:8081"),
	}
	return cfg, nil
}

// getEnv получает переменную окружения или значение по умолчанию
func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

// GetDatabaseURL формирует строку подключения к БД
func (c *Config) GetDatabaseURL() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s", c.DBUser, c.DBPassword, c.DBHost, c.DBPort, c.DBName, c.DBSSLMode)
}
