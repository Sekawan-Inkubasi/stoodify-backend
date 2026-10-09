package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Helpers parsing & validasi bersama antar controller.

func NoContent(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

func ValidationError(c *gin.Context, err error) {
	BadRequest(c, "Body request tidak valid", map[string]string{"body": err.Error()})
}

func ServerError(c *gin.Context, err error) {
	Fail(c, http.StatusInternalServerError, "mohon maaf ada kesalahan di sisi kami", map[string]string{
		"body": err.Error(),
	})
}
