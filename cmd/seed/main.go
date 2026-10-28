package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/HarmanSingh16/HMS-backend/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("unable to connect to db: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("database ping failed: %v", err)
	}
	log.Println("Connected to database successfully. Seeding data...")

	// 1. Generate default bcrypt hash for "Password123!"
	defaultPassword := "Password123!"
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(defaultPassword), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("failed to hash default password: %v", err)
	}
	passwordHash := string(hashedBytes)

	tx, err := pool.Begin(ctx)
	if err != nil {
		log.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback(ctx)

	// Clean up existing non-admin data to guarantee idempotency and clean state
	log.Println("Cleaning existing non-admin data...")
	cleanQueries := []string{
		"DELETE FROM notifications;",
		"DELETE FROM audit_logs;",
		"DELETE FROM appointment_status_events;",
		"DELETE FROM appointments;",
		"DELETE FROM doctor_time_off;",
		"DELETE FROM doctor_office_hours;",
		"DELETE FROM invites;",
		"DELETE FROM patients;",
		"DELETE FROM doctors;",
		"DELETE FROM departments;",
		"DELETE FROM accounts WHERE email != 'admin@hospital.com';",
	}
	for _, q := range cleanQueries {
		if _, err := tx.Exec(ctx, q); err != nil {
			log.Fatalf("clean query failed [%s]: %v", q, err)
		}
	}

	// 2. Ensure Admin Account exists
	var adminID int32
	err = tx.QueryRow(ctx, `
		INSERT INTO accounts (email, password_hash, role, status)
		VALUES ('admin@hospital.com', $1, 'admin', 'active')
		ON CONFLICT (email) DO UPDATE 
		SET password_hash = EXCLUDED.password_hash, status = 'active'
		RETURNING id;
	`, passwordHash).Scan(&adminID)
	if err != nil {
		log.Fatalf("failed to upsert admin account: %v", err)
	}
	log.Printf("Admin account ready: ID=%d, email=admin@hospital.com", adminID)

	// 3. Seed Departments (12 entries >= 10)
	departmentNames := []string{
		"Cardiology",
		"Neurology",
		"Orthopedics",
		"Pediatrics",
		"Dermatology",
		"Oncology",
		"Gastroenterology",
		"Ophthalmology",
		"Emergency Medicine",
		"General Surgery",
		"Psychiatry",
		"Radiology",
	}

	deptIDs := make([]int32, 0, len(departmentNames))
	for _, name := range departmentNames {
		var deptID int32
		err := tx.QueryRow(ctx, `
			INSERT INTO departments (name)
			VALUES ($1)
			RETURNING id;
		`, name).Scan(&deptID)
		if err != nil {
			log.Fatalf("failed to insert department %s: %v", name, err)
		}
		deptIDs = append(deptIDs, deptID)
	}
	log.Printf("Seeded %d departments", len(deptIDs))

	// 4. Seed Doctors (12 doctor accounts & doctor profiles >= 10)
	type doctorInfo struct {
		email        string
		firstName    string
		lastName     string
		deptIndex    int
		prefix       string
		specialtyDay int
	}

	doctorsData := []doctorInfo{
		{"dr.sarah.connor@hospital.com", "Sarah", "Connor", 0, "Dr. Connor", 1},
		{"dr.john.watson@hospital.com", "John", "Watson", 1, "Dr. Watson", 2},
		{"dr.gregory.house@hospital.com", "Gregory", "House", 2, "Dr. House", 3},
		{"dr.meredith.grey@hospital.com", "Meredith", "Grey", 3, "Dr. Grey", 4},
		{"dr.stephen.strange@hospital.com", "Stephen", "Strange", 4, "Dr. Strange", 5},
		{"dr.leonard.mccoy@hospital.com", "Leonard", "McCoy", 5, "Dr. McCoy", 1},
		{"dr.cristina.yang@hospital.com", "Cristina", "Yang", 6, "Dr. Yang", 2},
		{"dr.shaun.murphy@hospital.com", "Shaun", "Murphy", 7, "Dr. Murphy", 3},
		{"dr.allison.cameron@hospital.com", "Allison", "Cameron", 8, "Dr. Cameron", 4},
		{"dr.robert.chase@hospital.com", "Robert", "Chase", 9, "Dr. Chase", 5},
		{"dr.james.wilson@hospital.com", "James", "Wilson", 10, "Dr. Wilson", 1},
		{"dr.lisa.cuddy@hospital.com", "Lisa", "Cuddy", 11, "Dr. Cuddy", 2},
	}

	doctorAccountIDs := make([]int32, 0, len(doctorsData))
	for _, d := range doctorsData {
		var docAccID int32
		err := tx.QueryRow(ctx, `
			INSERT INTO accounts (email, password_hash, role, status)
			VALUES ($1, $2, 'doctor', 'active')
			RETURNING id;
		`, d.email, passwordHash).Scan(&docAccID)
		if err != nil {
			log.Fatalf("failed to insert doctor account %s: %v", d.email, err)
		}

		deptID := deptIDs[d.deptIndex]
		_, err = tx.Exec(ctx, `
			INSERT INTO doctors (account_id, department_id, first_name, last_name)
			VALUES ($1, $2, $3, $4);
		`, docAccID, deptID, d.firstName, d.lastName)
		if err != nil {
			log.Fatalf("failed to insert doctor profile %s: %v", d.email, err)
		}
		doctorAccountIDs = append(doctorAccountIDs, docAccID)
	}
	log.Printf("Seeded %d doctors", len(doctorAccountIDs))

	// 5. Seed Receptionists (3 receptionists)
	receptionistEmails := []struct {
		email  string
		status string
	}{
		{"reception.clara@hospital.com", "active"},
		{"reception.mark@hospital.com", "active"},
		{"reception.elena@hospital.com", "invited"},
	}
	receptionistIDs := make([]int32, 0, len(receptionistEmails))
	for _, r := range receptionistEmails {
		var recID int32
		err := tx.QueryRow(ctx, `
			INSERT INTO accounts (email, password_hash, role, status)
			VALUES ($1, $2, 'receptionist', $3)
			RETURNING id;
		`, r.email, passwordHash, r.status).Scan(&recID)
		if err != nil {
			log.Fatalf("failed to insert receptionist %s: %v", r.email, err)
		}
		receptionistIDs = append(receptionistIDs, recID)
	}
	mainReceptionistID := receptionistIDs[0]
	log.Printf("Seeded %d receptionists", len(receptionistIDs))

	// 6. Seed Patients (12 patient accounts & profiles >= 10)
	type patientInfo struct {
		email       string
		firstName   string
		lastName    string
		dob         string
		phone       string
		gender      string
		address     string
		status      string
		isClaimed   bool
	}

	patientsData := []patientInfo{
		{"alice.smith@example.com", "Alice", "Smith", "1990-05-14", "+1-555-0101", "Female", "742 Evergreen Terrace, Springfield", "active", true},
		{"bob.jones@example.com", "Bob", "Jones", "1985-11-23", "+1-555-0102", "Male", "124 Conch Street, Bikini Bottom", "active", true},
		{"charlie.brown@example.com", "Charlie", "Brown", "1978-02-17", "+1-555-0103", "Male", "456 Elm Street, Metropolis", "active", true},
		{"diana.prince@example.com", "Diana", "Prince", "1992-08-30", "+1-555-0104", "Female", "100 Gateway Boulevard, Gotham", "active", true},
		{"evan.wright@example.com", "Evan", "Wright", "1965-12-05", "+1-555-0105", "Male", "88 Ocean Drive, Miami", "active", true},
		{"fiona.gallagher@example.com", "Fiona", "Gallagher", "1995-07-19", "+1-555-0106", "Female", "2114 Canary Lane, Chicago", "active", true},
		{"george.clark@example.com", "George", "Clark", "1988-03-25", "+1-555-0107", "Male", "302 Baker Street, London", "active", true},
		{"hannah.abbott@example.com", "Hannah", "Abbott", "2001-09-12", "+1-555-0108", "Female", "55 Maple Avenue, Seattle", "active", true},
		{"ian.malcolm@example.com", "Ian", "Malcolm", "1972-06-18", "+1-555-0109", "Male", "42 Jurassic Way, Austin", "active", true},
		{"julia.roberts@example.com", "Julia", "Roberts", "1980-10-28", "+1-555-0110", "Female", "17 Beverly Crest, Los Angeles", "active", true},
		{"kevin.bacon@example.com", "Kevin", "Bacon", "1983-04-01", "+1-555-0111", "Male", "6 Degrees Circle, Philadelphia", "invited", false},
		{"laura.croft@example.com", "Laura", "Croft", "1991-01-14", "+1-555-0112", "Female", "1 Croft Manor, Surrey", "deactivated", true},
	}

	patientAccountIDs := make([]int32, 0, len(patientsData))
	for _, p := range patientsData {
		var patAccID int32
		err := tx.QueryRow(ctx, `
			INSERT INTO accounts (email, password_hash, role, status)
			VALUES ($1, $2, 'patient', $3)
			RETURNING id;
		`, p.email, passwordHash, p.status).Scan(&patAccID)
		if err != nil {
			log.Fatalf("failed to insert patient account %s: %v", p.email, err)
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO patients (account_id, date_of_birth, first_name, last_name, phone_number, gender, address)
			VALUES ($1, $2::date, $3, $4, $5, $6, $7);
		`, patAccID, p.dob, p.firstName, p.lastName, p.phone, p.gender, p.address)
		if err != nil {
			log.Fatalf("failed to insert patient demographics %s: %v", p.email, err)
		}
		patientAccountIDs = append(patientAccountIDs, patAccID)
	}
	log.Printf("Seeded %d patients", len(patientAccountIDs))

	// 7. Seed Invites (13 entries >= 10)
	// Create invites for patients + 1 invited receptionist
	inviteIDs := make([]int32, 0)
	type inviteSeed struct {
		accountID int32
		token     string
		claimed   bool
	}
	invitesToCreate := make([]inviteSeed, 0)
	for i, p := range patientsData {
		invitesToCreate = append(invitesToCreate, inviteSeed{
			accountID: patientAccountIDs[i],
			token:     fmt.Sprintf("patient_invite_token_sample_%d_%s", i+1, p.firstName),
			claimed:   p.isClaimed,
		})
	}
	invitesToCreate = append(invitesToCreate, inviteSeed{
		accountID: receptionistIDs[2], // Elena Gilbert (invited)
		token:     "receptionist_elena_invite_token_123",
		claimed:   false,
	})

	for _, inv := range invitesToCreate {
		th := hashToken(inv.token)
		var claimedAt *time.Time
		if inv.claimed {
			t := time.Now().Add(-24 * time.Hour)
			claimedAt = &t
		}
		expiresAt := time.Now().Add(48 * time.Hour)

		var invID int32
		err := tx.QueryRow(ctx, `
			INSERT INTO invites (account_id, token_hash, expires_at, claimed_at)
			VALUES ($1, $2, $3, $4)
			RETURNING id;
		`, inv.accountID, th, expiresAt, claimedAt).Scan(&invID)
		if err != nil {
			log.Fatalf("failed to insert invite: %v", err)
		}
		inviteIDs = append(inviteIDs, invID)
	}
	log.Printf("Seeded %d invites", len(inviteIDs))

	// 8. Seed Doctor Office Hours (24 entries >= 10)
	// Monday (1) to Friday (5) office hours across doctors
	officeHoursCount := 0
	for i, docID := range doctorAccountIDs {
		// Doctor gets 2 recurring weekdays
		w1 := int16((i % 5) + 1)
		w2 := int16(((i + 2) % 5) + 1)

		_, err := tx.Exec(ctx, `
			INSERT INTO doctor_office_hours (doctor_id, weekday, start_time, end_time)
			VALUES ($1, $2, '09:00:00', '17:00:00');
		`, docID, w1)
		if err != nil {
			log.Fatalf("failed to insert doctor office hours: %v", err)
		}
		officeHoursCount++

		_, err = tx.Exec(ctx, `
			INSERT INTO doctor_office_hours (doctor_id, weekday, start_time, end_time)
			VALUES ($1, $2, '10:00:00', '16:00:00');
		`, docID, w2)
		if err != nil {
			log.Fatalf("failed to insert doctor office hours: %v", err)
		}
		officeHoursCount++
	}
	log.Printf("Seeded %d doctor office hours", officeHoursCount)

	// 9. Seed Doctor Time Off (12 entries >= 10)
	reasons := []string{
		"Annual Medical Conference",
		"Continuing Medical Education (CME)",
		"Personal Family Leave",
		"Annual Vacation",
		"Medical Board Review",
		"Research Fellowship Summit",
		"Symposium on Advanced Diagnostics",
		"Surgical Skills Workshop",
		"Departmental Retreat",
		"Personal Emergency Leave",
		"Clinical Research Blackout",
		"Guest Lecturer at Medical School",
	}

	for i, docID := range doctorAccountIDs {
		starts := time.Now().AddDate(0, 0, (i*3)+2)
		ends := starts.AddDate(0, 0, 2)
		_, err := tx.Exec(ctx, `
			INSERT INTO doctor_time_off (doctor_id, starts_at, ends_at, reason)
			VALUES ($1, $2, $3, $4);
		`, docID, starts, ends, reasons[i])
		if err != nil {
			log.Fatalf("failed to insert doctor time off: %v", err)
		}
	}
	log.Printf("Seeded %d doctor time off records", len(doctorAccountIDs))

	// 10. Seed Appointments (15 entries >= 10)
	// All appointments are scheduled on non-overlapping slots per doctor
	now := time.Now().Truncate(time.Hour)
	type apptSeed struct {
		docIdx   int
		patIdx   int
		startsAt time.Time
		endsAt   time.Time
		status   string
		reason   *string
	}

	cancelReason1 := "Patient had a conflicting work commitment"
	cancelReason2 := "Emergency surgery required doctor rescheduling"

	apptsData := []apptSeed{
		// Completed past appointments
		{0, 0, now.AddDate(0, 0, -4).Add(9 * time.Hour), now.AddDate(0, 0, -4).Add(9*time.Hour + 30*time.Minute), "completed", nil},
		{0, 1, now.AddDate(0, 0, -4).Add(11 * time.Hour), now.AddDate(0, 0, -4).Add(11*time.Hour + 30*time.Minute), "completed", nil},
		{1, 2, now.AddDate(0, 0, -3).Add(10 * time.Hour), now.AddDate(0, 0, -3).Add(10*time.Hour + 45*time.Minute), "completed", nil},
		{2, 3, now.AddDate(0, 0, -3).Add(14 * time.Hour), now.AddDate(0, 0, -3).Add(14*time.Hour + 30*time.Minute), "completed", nil},
		{3, 4, now.AddDate(0, 0, -2).Add(15 * time.Hour), now.AddDate(0, 0, -2).Add(15*time.Hour + 30*time.Minute), "completed", nil},
		// Checked-in appointments today
		{4, 5, now.Add(-1 * time.Hour), now.Add(-1*time.Hour + 30*time.Minute), "checked_in", nil},
		{5, 6, now.Add(1 * time.Hour), now.Add(1*time.Hour + 30*time.Minute), "checked_in", nil},
		{6, 7, now.Add(2 * time.Hour), now.Add(2*time.Hour + 30*time.Minute), "checked_in", nil},
		// Booked upcoming appointments
		{7, 8, now.AddDate(0, 0, 1).Add(9 * time.Hour), now.AddDate(0, 0, 1).Add(9*time.Hour + 30*time.Minute), "booked", nil},
		{8, 9, now.AddDate(0, 0, 1).Add(10 * time.Hour), now.AddDate(0, 0, 1).Add(10*time.Hour + 30*time.Minute), "booked", nil},
		{9, 0, now.AddDate(0, 0, 2).Add(14 * time.Hour), now.AddDate(0, 0, 2).Add(14*time.Hour + 30*time.Minute), "booked", nil},
		{10, 1, now.AddDate(0, 0, 2).Add(15 * time.Hour), now.AddDate(0, 0, 2).Add(15*time.Hour + 30*time.Minute), "booked", nil},
		{11, 2, now.AddDate(0, 0, 3).Add(11 * time.Hour), now.AddDate(0, 0, 3).Add(11*time.Hour + 30*time.Minute), "booked", nil},
		// Cancelled & No-show
		{0, 3, now.AddDate(0, 0, -5).Add(14 * time.Hour), now.AddDate(0, 0, -5).Add(14*time.Hour + 30*time.Minute), "cancelled", &cancelReason1},
		{1, 4, now.AddDate(0, 0, -1).Add(16 * time.Hour), now.AddDate(0, 0, -1).Add(16*time.Hour + 30*time.Minute), "no_show", nil},
		{2, 5, now.AddDate(0, 0, 4).Add(15 * time.Hour), now.AddDate(0, 0, 4).Add(15*time.Hour + 30*time.Minute), "cancelled", &cancelReason2},
	}

	type createdAppt struct {
		id        int32
		status    string
		patientID int32
		doctorID  int32
	}
	createdAppts := make([]createdAppt, 0, len(apptsData))

	for _, a := range apptsData {
		docID := doctorAccountIDs[a.docIdx]
		patID := patientAccountIDs[a.patIdx]

		var apptID int32
		err := tx.QueryRow(ctx, `
			INSERT INTO appointments (doctor_id, patient_id, starts_at, ends_at, status, cancellation_reason)
			VALUES ($1, $2, $3, $4, $5, $6)
			RETURNING id;
		`, docID, patID, a.startsAt, a.endsAt, a.status, a.reason).Scan(&apptID)
		if err != nil {
			log.Fatalf("failed to insert appointment: %v", err)
		}
		createdAppts = append(createdAppts, createdAppt{
			id:        apptID,
			status:    a.status,
			patientID: patID,
			doctorID:  docID,
		})
	}
	log.Printf("Seeded %d appointments", len(createdAppts))

	// 11. Seed Appointment Status Events (>= 15 entries)
	statusEventsCount := 0
	for _, appt := range createdAppts {
		// Initial booked event
		_, err := tx.Exec(ctx, `
			INSERT INTO appointment_status_events (appointment_id, from_status, to_status, changed_by, reason)
			VALUES ($1, NULL, 'booked', $2, 'Appointment initially booked');
		`, appt.id, mainReceptionistID)
		if err != nil {
			log.Fatalf("failed to insert status event: %v", err)
		}
		statusEventsCount++

		switch appt.status {
		case "checked_in":
			_, err = tx.Exec(ctx, `
				INSERT INTO appointment_status_events (appointment_id, from_status, to_status, changed_by, reason)
				VALUES ($1, 'booked', 'checked_in', $2, 'Patient arrived at front desk');
			`, appt.id, mainReceptionistID)
			if err != nil {
				log.Fatalf("failed to insert status event: %v", err)
			}
			statusEventsCount++

		case "completed":
			_, err = tx.Exec(ctx, `
				INSERT INTO appointment_status_events (appointment_id, from_status, to_status, changed_by, reason)
				VALUES ($1, 'booked', 'checked_in', $2, 'Patient arrived at front desk');
			`, appt.id, mainReceptionistID)
			if err != nil {
				log.Fatalf("failed to insert status event: %v", err)
			}
			statusEventsCount++

			_, err = tx.Exec(ctx, `
				INSERT INTO appointment_status_events (appointment_id, from_status, to_status, changed_by, reason)
				VALUES ($1, 'checked_in', 'completed', $2, 'Doctor concluded consultation and examination');
			`, appt.id, appt.doctorID)
			if err != nil {
				log.Fatalf("failed to insert status event: %v", err)
			}
			statusEventsCount++

		case "cancelled":
			_, err = tx.Exec(ctx, `
				INSERT INTO appointment_status_events (appointment_id, from_status, to_status, changed_by, reason)
				VALUES ($1, 'booked', 'cancelled', $2, 'Patient called clinic to cancel');
			`, appt.id, appt.patientID)
			if err != nil {
				log.Fatalf("failed to insert status event: %v", err)
			}
			statusEventsCount++

		case "no_show":
			_, err = tx.Exec(ctx, `
				INSERT INTO appointment_status_events (appointment_id, from_status, to_status, changed_by, reason)
				VALUES ($1, 'booked', 'no_show', $2, 'Automatic no-show detection: patient did not check in within grace period');
			`, appt.id, adminID)
			if err != nil {
				log.Fatalf("failed to insert status event: %v", err)
			}
			statusEventsCount++
		}
	}
	log.Printf("Seeded %d appointment status events", statusEventsCount)

	// 12. Seed Audit Logs (15 entries >= 10)
	type auditSeed struct {
		actorID    int32
		action     string
		entityType string
		entityID   int32
		details    map[string]any
	}
	auditList := []auditSeed{
		{adminID, "department_created", "department", deptIDs[0], map[string]any{"name": "Cardiology", "created_by": "admin"}},
		{adminID, "department_created", "department", deptIDs[1], map[string]any{"name": "Neurology", "created_by": "admin"}},
		{adminID, "department_created", "department", deptIDs[2], map[string]any{"name": "Orthopedics", "created_by": "admin"}},
		{mainReceptionistID, "patient_registered", "patient", patientAccountIDs[0], map[string]any{"name": "Alice Smith", "phone": "+1-555-0101"}},
		{mainReceptionistID, "patient_registered", "patient", patientAccountIDs[1], map[string]any{"name": "Bob Jones", "phone": "+1-555-0102"}},
		{mainReceptionistID, "patient_registered", "patient", patientAccountIDs[2], map[string]any{"name": "Charlie Brown", "phone": "+1-555-0103"}},
		{mainReceptionistID, "appointment_booked", "appointment", createdAppts[0].id, map[string]any{"doctor_id": createdAppts[0].doctorID, "patient_id": createdAppts[0].patientID}},
		{mainReceptionistID, "appointment_booked", "appointment", createdAppts[1].id, map[string]any{"doctor_id": createdAppts[1].doctorID, "patient_id": createdAppts[1].patientID}},
		{mainReceptionistID, "appointment_checked_in", "appointment", createdAppts[5].id, map[string]any{"status": "checked_in", "desk": "Counter 1"}},
		{doctorAccountIDs[0], "appointment_completed", "appointment", createdAppts[0].id, map[string]any{"notes_recorded": true, "status": "completed"}},
		{doctorAccountIDs[1], "appointment_completed", "appointment", createdAppts[2].id, map[string]any{"notes_recorded": true, "status": "completed"}},
		{doctorAccountIDs[0], "time_off_created", "doctor_time_off", doctorAccountIDs[0], map[string]any{"reason": "Conference", "duration_days": 2}},
		{adminID, "account_deactivated", "account", patientAccountIDs[11], map[string]any{"email": "laura.croft@example.com", "reason": "Patient requested archival"}},
		{mainReceptionistID, "appointment_cancelled", "appointment", createdAppts[13].id, map[string]any{"cancellation_reason": cancelReason1}},
		{adminID, "system_settings_updated", "system", 1, map[string]any{"maintenance_window": "Sunday 02:00 UTC", "notifications_enabled": true}},
	}

	for _, a := range auditList {
		detJSON, _ := json.Marshal(a.details)
		_, err := tx.Exec(ctx, `
			INSERT INTO audit_logs (actor_id, action, entity_type, entity_id, details)
			VALUES ($1, $2, $3, $4, $5);
		`, a.actorID, a.action, a.entityType, a.entityID, detJSON)
		if err != nil {
			log.Fatalf("failed to insert audit log: %v", err)
		}
	}
	log.Printf("Seeded %d audit logs", len(auditList))

	// 13. Seed Notifications (15 entries >= 10)
	type notifSeed struct {
		recipientID   int32
		inviteID      *int32
		appointmentID *int32
		notifType     string
		status        string
		payload       map[string]any
		scheduledFor  time.Time
		sentAt        *time.Time
	}

	pastSentTime := time.Now().Add(-2 * time.Hour)
	futureSchedTime := time.Now().Add(12 * time.Hour)

	notifsList := []notifSeed{
		// 8 Account Invites
		{patientAccountIDs[0], &inviteIDs[0], nil, "account_invite", "sent", map[string]any{"email": "alice.smith@example.com", "subject": "Welcome to Hospital Management Portal"}, time.Now().Add(-24 * time.Hour), &pastSentTime},
		{patientAccountIDs[1], &inviteIDs[1], nil, "account_invite", "sent", map[string]any{"email": "bob.jones@example.com", "subject": "Welcome to Hospital Management Portal"}, time.Now().Add(-24 * time.Hour), &pastSentTime},
		{patientAccountIDs[2], &inviteIDs[2], nil, "account_invite", "sent", map[string]any{"email": "charlie.brown@example.com", "subject": "Welcome to Hospital Management Portal"}, time.Now().Add(-24 * time.Hour), &pastSentTime},
		{patientAccountIDs[3], &inviteIDs[3], nil, "account_invite", "sent", map[string]any{"email": "diana.prince@example.com", "subject": "Welcome to Hospital Management Portal"}, time.Now().Add(-24 * time.Hour), &pastSentTime},
		{patientAccountIDs[4], &inviteIDs[4], nil, "account_invite", "sent", map[string]any{"email": "evan.wright@example.com", "subject": "Welcome to Hospital Management Portal"}, time.Now().Add(-24 * time.Hour), &pastSentTime},
		{patientAccountIDs[10], &inviteIDs[10], nil, "account_invite", "pending", map[string]any{"email": "kevin.bacon@example.com", "subject": "Set up your patient portal account"}, time.Now(), nil},
		{receptionistIDs[2], &inviteIDs[12], nil, "account_invite", "pending", map[string]any{"email": "reception.elena@hospital.com", "subject": "Staff invitation to HMS Portal"}, time.Now(), nil},
		{patientAccountIDs[5], &inviteIDs[5], nil, "account_invite", "sent", map[string]any{"email": "fiona.gallagher@example.com", "subject": "Welcome to Hospital Management Portal"}, time.Now().Add(-12 * time.Hour), &pastSentTime},
		// 5 Appointment Reminders
		{createdAppts[8].patientID, nil, &createdAppts[8].id, "appointment_reminder", "pending", map[string]any{"message": "Reminder: Upcoming doctor appointment tomorrow at 09:00 AM"}, futureSchedTime, nil},
		{createdAppts[9].patientID, nil, &createdAppts[9].id, "appointment_reminder", "pending", map[string]any{"message": "Reminder: Upcoming doctor appointment tomorrow at 10:00 AM"}, futureSchedTime, nil},
		{createdAppts[0].patientID, nil, &createdAppts[0].id, "appointment_reminder", "sent", map[string]any{"message": "Appointment reminder for Cardiology consultation"}, time.Now().Add(-5 * 24 * time.Hour), &pastSentTime},
		{createdAppts[1].patientID, nil, &createdAppts[1].id, "appointment_reminder", "sent", map[string]any{"message": "Appointment reminder for Follow-up consultation"}, time.Now().Add(-5 * 24 * time.Hour), &pastSentTime},
		{createdAppts[2].patientID, nil, &createdAppts[2].id, "appointment_reminder", "sent", map[string]any{"message": "Appointment reminder for Neurology evaluation"}, time.Now().Add(-4 * 24 * time.Hour), &pastSentTime},
		// 2 Slot Removed / Schedule Change Notifications
		{createdAppts[13].patientID, nil, &createdAppts[13].id, "slot_removed", "sent", map[string]any{"message": "Your appointment slot was cancelled per your request"}, time.Now().Add(-5 * 24 * time.Hour), &pastSentTime},
		{createdAppts[14].patientID, nil, &createdAppts[14].id, "slot_removed", "sent", map[string]any{"message": "Appointment slot updated due to emergency doctor duty"}, time.Now().Add(-24 * time.Hour), &pastSentTime},
	}

	for _, n := range notifsList {
		payloadJSON, _ := json.Marshal(n.payload)
		_, err := tx.Exec(ctx, `
			INSERT INTO notifications (recipient_id, invite_id, appointment_id, type, status, payload, scheduled_for, sent_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8);
		`, n.recipientID, n.inviteID, n.appointmentID, n.notifType, n.status, payloadJSON, n.scheduledFor, n.sentAt)
		if err != nil {
			log.Fatalf("failed to insert notification: %v", err)
		}
	}
	log.Printf("Seeded %d notifications", len(notifsList))

	if err := tx.Commit(ctx); err != nil {
		log.Fatalf("failed to commit transaction: %v", err)
	}
	log.Println("Seeding transaction committed successfully!")

	// 14. Verification of Table Counts
	tables := []string{
		"departments",
		"accounts",
		"doctors",
		"patients",
		"invites",
		"doctor_office_hours",
		"doctor_time_off",
		"appointments",
		"appointment_status_events",
		"audit_logs",
		"notifications",
	}

	fmt.Println("\n================ FINAL VERIFICATION: TABLE COUNTS ================")
	allAboveTen := true
	for _, t := range tables {
		var count int
		q := fmt.Sprintf("SELECT COUNT(*) FROM %s", t)
		if err := pool.QueryRow(ctx, q).Scan(&count); err != nil {
			fmt.Printf("ERROR checking %s: %v\n", t, err)
			allAboveTen = false
		} else {
			statusMark := "✅ PASS"
			if count < 10 {
				statusMark = "❌ FAIL (< 10)"
				allAboveTen = false
			}
			fmt.Printf(" %-28s : %3d rows  [%s]\n", t, count, statusMark)
		}
	}
	fmt.Println("==================================================================")

	if allAboveTen {
		fmt.Println("SUCCESS: All tables have at least 10 rows!")
	} else {
		log.Fatalf("FAILURE: Not all tables met the >= 10 rows threshold!")
	}
}
