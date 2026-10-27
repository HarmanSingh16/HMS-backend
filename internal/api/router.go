package api

import (
	"net/http"

	"github.com/HarmanSingh16/HMS-backend/internal/api/handler"
	"github.com/HarmanSingh16/HMS-backend/internal/api/middleware"
	"github.com/HarmanSingh16/HMS-backend/internal/config"
	"github.com/HarmanSingh16/HMS-backend/internal/domain"
	"github.com/gin-gonic/gin"
)

type RouterConfig struct {
	Config         *config.Config
	AuthHandler    *handler.AuthHandler
	AdminHandler   *handler.AdminHandler
	PatientHandler *handler.PatientHandler
}

func SetupRouter(rc *RouterConfig) *gin.Engine {
	r := gin.Default()

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	v1 := r.Group("/api/v1")
	{
		// Auth routes
		authGroup := v1.Group("/auth")
		{
			// Public routes
			authGroup.POST("/login", rc.AuthHandler.Login)
			authGroup.POST("/refresh", rc.AuthHandler.RefreshToken)
			authGroup.GET("/invite/verify", rc.AuthHandler.VerifyInvite)
			authGroup.POST("/invite/claim", rc.AuthHandler.ClaimInvite)

			// Protected routes
			protectedAuth := authGroup.Group("")
			protectedAuth.Use(middleware.Auth(rc.Config.JWTSecret))
			{
				protectedAuth.POST("/logout", rc.AuthHandler.Logout)
				protectedAuth.GET("/me", rc.AuthHandler.GetMe)
			}
		}

		// top level departments resource
		deptGroup := v1.Group("/departments")
		deptGroup.Use(middleware.Auth(rc.Config.JWTSecret))
		{
			// Any staff member can view departments
			deptGroup.GET("",
				middleware.RequireRoles(domain.RoleAdmin, domain.RoleReceptionist, domain.RoleDoctor),
				rc.AdminHandler.ListDepartments,
			)
			// Only Admin can create or modify departments
			deptGroup.POST("",
				middleware.RequireRoles(domain.RoleAdmin),
				rc.AdminHandler.CreateDepartment,
			)
			deptGroup.PUT("/:id",
				middleware.RequireRoles(domain.RoleAdmin),
				rc.AdminHandler.UpdateDepartment,
			)
		}

		// Receptionist only routes
		receptionistGroup := v1.Group("/receptionist")
		receptionistGroup.Use(middleware.Auth(rc.Config.JWTSecret))
		receptionistGroup.Use(middleware.RequireRoles(domain.RoleReceptionist))
		{
			receptionistGroup.POST(
				"/patients",
				rc.PatientHandler.RegisterPatient,
			)
		}

		// Patient resource routes
		patientGroup := v1.Group("/patients")
		patientGroup.Use(middleware.Auth(rc.Config.JWTSecret))
		{
			patientGroup.GET(
				"",
				middleware.RequireRoles(domain.RoleReceptionist, domain.RoleDoctor, domain.RoleAdmin),
				rc.PatientHandler.SearchPatients,
			)

			patientGroup.GET(
				"/:id",
				middleware.RequireRoles(domain.RoleAdmin, domain.RolePatient, domain.RoleDoctor),
				rc.PatientHandler.GetPatient,
			)

			patientGroup.PUT("/:id",
				middleware.RequireRoles(domain.RoleReceptionist, domain.RolePatient),
				rc.PatientHandler.UpdatePatient,
			)
		}
	}
	return r
}
