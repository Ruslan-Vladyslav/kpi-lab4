package main

import (
    "testing"

    "github.com/stretchr/testify/assert"
)

func TestHealthFunction(t *testing.T) {
    // вызываем функцию health и проверяем результат
    result := health("server1:8080")
    assert.IsType(t, true, result) 
}
