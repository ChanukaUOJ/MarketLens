package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (ctrl *JobController) GetAllJobTypesHandler(c *gin.Context) {
	items, err := ctrl.repo.GetAllJobTypes()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch job types", "details": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"count": len(items), "job_types": items})
}
