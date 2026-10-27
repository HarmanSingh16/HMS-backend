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
	Config             *config.Config
	AuthHandler        *handler.AuthHandler
	AdminHandler       *handler.AdminHandler
	PatientHandler     *handler.PatientHandler
	DoctorHandler      *handler.DoctorHandler
	AppointmentHandler *handler.AppointmentHandler
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

		// // Doctor & Schedule resource routes
		doctorGroup := v1.Group("/doctors")
		doctorGroup.Use(middleware.Auth(rc.Config.JWTSecret))
		{
			// All authenticated roles can list doctors & view their schedule
			doctorGroup.GET("", rc.DoctorHandler.ListDoctors)
			doctorGroup.GET("/:id/schedule", rc.DoctorHandler.GetDoctorSchedule)
			// Slot projection available to Receptionist, Doctor, Patient, Admin
			doctorGroup.GET(
				"/:id/available-slots",
				middleware.RequireRoles(domain.RoleReceptionist, domain.RoleDoctor, domain.RolePatient, domain.RoleAdmin),
				rc.DoctorHandler.GetAvailableSlots,
			)
			// Office hours & time-off: Doctor or Admin
			doctorGroup.PUT(
				"/:id/office-hours",
				middleware.RequireRoles(domain.RoleDoctor, domain.RoleAdmin),
				rc.DoctorHandler.UpdateOfficeHours,
			)
			doctorGroup.POST(
				"/:id/time-off",
				middleware.RequireRoles(domain.RoleDoctor, domain.RoleAdmin),
				rc.DoctorHandler.CreateTimeOff,
			)
		}

		// Appointments Lifecycle routes
		appointmentGroup := v1.Group("/appointments")
		appointmentGroup.Use(middleware.Auth(rc.Config.JWTSecret))
		{
			// Booking: Receptionist only
			appointmentGroup.POST("",
				middleware.RequireRoles(domain.RoleReceptionist),
				rc.AppointmentHandler.BookAppointment,
			)
			// Listing appointments: All authenticated roles (scoped inside service)
			appointmentGroup.GET("",
				middleware.RequireRoles(domain.RoleReceptionist, domain.RoleDoctor, domain.RolePatient, domain.RoleAdmin),
				rc.AppointmentHandler.ListAppointments,
			)
			// View single appointment: Scoped
			appointmentGroup.GET("/:id",
				middleware.RequireRoles(domain.RoleReceptionist, domain.RoleDoctor, domain.RolePatient, domain.RoleAdmin),
				rc.AppointmentHandler.GetAppointment,
			)
			// Check-in: Receptionist only
			appointmentGroup.POST("/:id/check-in",
				middleware.RequireRoles(domain.RoleReceptionist),
				rc.AppointmentHandler.CheckIn,
			)
			// Complete: Doctor, Receptionist, Admin
			appointmentGroup.POST("/:id/complete",
				middleware.RequireRoles(domain.RoleDoctor, domain.RoleReceptionist, domain.RoleAdmin),
				rc.AppointmentHandler.Complete,
			)
			// Cancel: Patient (up to 1h), Staff (any time)
			appointmentGroup.POST("/:id/cancel",
				middleware.RequireRoles(domain.RolePatient, domain.RoleDoctor, domain.RoleReceptionist, domain.RoleAdmin),
				rc.AppointmentHandler.Cancel,
			)
			// Revert No-Show: Receptionist, Doctor, Admin
			appointmentGroup.POST("/:id/revert-no-show",
				middleware.RequireRoles(domain.RoleReceptionist, domain.RoleDoctor, domain.RoleAdmin),
				rc.AppointmentHandler.RevertNoShow,
			)
			// Transition timeline
			appointmentGroup.GET("/:id/history",
				middleware.RequireRoles(domain.RoleReceptionist, domain.RoleDoctor, domain.RolePatient, domain.RoleAdmin),
				rc.AppointmentHandler.GetHistory,
			)
		}
	}
	return r
}
