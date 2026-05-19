package main

import (
	"net/http"
	"github.com/gin-gonic/gin"
)

func main() {
	r := gin.Default()
	r.GET("/users", listUsers)
	r.POST("/users", createUser)
	r.GET("/users/:id", getUser)
	http.ListenAndServe(":8080", r)
}

func listUsers(c *gin.Context) {
	c.JSON(200, gin.H{"users": []string{}})
}

func createUser(c *gin.Context) {
	c.JSON(201, gin.H{"id": "new"})
}

func getUser(c *gin.Context) {
	id := c.Param("id")
	c.JSON(200, gin.H{"id": id})
}
