# Hospital Management System: Project Requirements

Oct 10, 2026 · @Harman Singh

## Overview and goals

This is a single-hospital system where patients, doctors, receptionists and admins work against shared appointment and medical data, and clinical data is visible only to the treating doctor and the patient. It is a personal portfolio project, built with a Go backend and a React frontend, to show backend engineering depth when applying for roles.

The aim is depth over breadth: effort goes into a few hard problems and into decisions that can be defended in an interview and explained in the README.

- **Users:** patients, doctors, receptionists (who also act as nurses) and admins.
- **Covers:** appointment booking and cancellation, per-visit medical records, structured prescriptions, vitals and email notifications.
- **Must prove:** correct concurrent booking, fine-grained authorization over sensitive data and reliable background email.

## Core engineering challenges

Three problems carry the project; the rest is routine CRUD.

1. **Concurrent booking.** Two staff members booking the same 15-minute slot at the same moment must produce exactly one booking. The same holds when a freed slot is rebooked right after a cancellation. A concurrency test should demonstrate it.
2. **Authorization over sensitive data.** A role check is not enough: access to clinical data depends on the relationship between the user and the patient, and every access is logged.
3. **Reliable time-based background work.** The 24-hour reminder and the slot-removed email must survive restarts, retry on failure and be cancelled when an appointment is cancelled.

## Roles and permissions

Clinical data (diagnosis, notes, prescriptions) is visible only to the patient and to a doctor who has started a consultation with that patient. Admin and receptionist have no path to it.

| Data | Patient | Doctor | Receptionist (also nurse) | Admin |
| --- | --- | --- | --- | --- |
| Identity and contact | Own | Patients on their appointments | Read, create (registers patients) | Manage accounts |
| Appointments | View own, cancel own | View own, cancel own | View all, book, cancel | Change appointments |
| Vitals | Read own | Read, for patients on their appointments | Read, write (editable until the consultation starts) | None |
| Clinical data | Read own | Read whole chart, append (after starting a consultation) | None | None |

Patients cannot book. Booking is done by the receptionist, who also records the vitals at the visit.

## Accounts and organisation

Every account has exactly one role, fixed at creation, and nobody sets another person's password.

- **Invites:** staff and patients are invited by email and set their own password through the link, so admin never knows anyone's credentials.
- **One role per account:** to change a role, deactivate the account and create a new one. An admin cannot promote themselves.
- **Patient registration:** the receptionist registers a patient, and a patient email is required. The patient later claims an account through the emailed invite, so a patient record can exist before a login does.
- **First admin:** seeded through a setup command. Further admins are invited by an existing admin.
- **Departments:** admin manages departments. Each doctor belongs to exactly one, and the receptionist narrows by department when booking.
- **Deactivation:** departing staff are deactivated, never deleted, so records and the audit trail stay intact. A deactivated doctor loses all access, but their authored entries stay in patient charts.

## Appointments

A slot is a fixed 15 minutes inside a doctor's own office hours, and nothing can be booked outside them.

- **Office hours:** each doctor sets their own weekly schedule. Slots are always 15 minutes.
- **Who books:** the receptionist. Patients cannot book.
- **Booking window:** up to 1 week ahead, with no minimum lead time.
- **No double-booking:** two simultaneous bookings for the same doctor and slot must result in exactly one booking.
- **Cancellation by patients:** allowed up to 1 hour before the slot.
- **Schedule changes:** a doctor's change applies immediately. Appointments that no longer fall inside their hours are cancelled, and the affected patients are emailed that their slot was removed.

## Appointment lifecycle

An appointment moves through five steps to completion, and the chart unlocks only when the doctor starts the consultation.

&#91;embedded content: appointment lifecycle · 7 states\]

A booked appointment ends as cancelled or no-show when the visit does not happen, and neither ever gets a record. Each note shows who moves the appointment into that state.

- **No-show:** marked automatically, and only if the patient never checked in by the end of the slot. A patient who is checked in and waiting for a late doctor stays in the queue.
- **Cancellation:** patients up to 1 hour before the slot; receptionist, doctor and admin at any time; the system when a schedule change removes the slot.
- **Reassigning** an appointment to another doctor grants that doctor nothing. Access comes only from starting the consultation.

## Medical records, vitals and prescriptions

Each consultation produces one record tied to its appointment, and records are append-only.

- **Creation:** a record exists from the moment the doctor starts the consultation. Cancelled and no-show appointments never get one.
- **Whole chart:** once a doctor has started a consultation with a patient, they can read that patient's whole chart, including other doctors' records. Access does not expire.
- **Writing:** the doctor writes during the consultation. After the visit they can add addenda, clearly labelled as amendments.
- **Append-only:** entries are never edited or deleted. A mistake is corrected with an amendment that references the original, and both stay visible.
- **Prescriptions:** structured line items on a record (drug name as free text, dosage, frequency, duration), downloadable as a PDF.
- **Vitals:** a fixed set of BP, pulse, temperature, weight and SpO2, each tied to an appointment. The receptionist can edit them until the consultation starts, and after that they are locked.
- **Patient view:** a timeline of the patient's visits, each opening to its record.

## Notifications

Email is sent in exactly three cases and nowhere else.

| Email | To | When |
| --- | --- | --- |
| Account invite | New patients, doctors, receptionists and further admins | When the account is created |
| Appointment reminder | Patient | 24 hours before the slot |
| Slot removed | Patient | When a doctor's schedule change cancels their appointment |

Reminder rules:

- No reminder is sent if the appointment is booked less than 24 hours ahead.
- The pending reminder is cancelled when the appointment is cancelled or removed.
- It is sent once, with retries on failure, and abandoned once the slot has passed.
- A patient email is required at registration, since the invite and the reminder both depend on it.

There are no booking confirmations, no cancellation notices and no emails to doctors.

## Security, audit and threat model

The system defends against a malicious application admin, not against someone with direct access to the database or server. Admin has no path to clinical data, and these controls keep it that way:

| Control | What it blocks |
| --- | --- |
| Invite-based onboarding, so admin never knows credentials | Puppet doctor accounts controlled by an admin |
| One role per account, fixed at creation | An admin promoting themselves to doctor |
| Chart access starts only when the doctor starts a consultation | Reassigning an appointment as a backdoor to a chart |
| Append-only audit log | Hiding abuse after the fact |
| Patient-visible access log | Abuse that slips through, because the affected patient can see it |

The audit log records every read or write of clinical data, account creation, schedule change and appointment cancellation. Patients can see who viewed their chart and when.

One accepted consequence: once granted, a doctor's chart access never expires, so the audit log and the patient-visible log carry the accountability.

## Out of scope

This version leaves out everything beyond the core flows above:

- Billing, insurance and payments
- Pharmacy and drug databases (drug names are free text)
- Lab results and imaging
- Inpatient beds and admissions
- Multiple hospitals or branches
- Patients booking for themselves
- Emails beyond the three listed

## Assumptions and open items

These were proposed during planning and not explicitly confirmed, so they are treated as agreed unless changed.

- Receptionist, doctor and admin can cancel an appointment at any time. Only patients have the 1-hour cutoff.
- A slot stays bookable until it ends, and a booking made during the slot is checked in immediately, so walk-ins work.
- Only the original author can amend a record entry.
- The doctor cannot start a consultation until the receptionist has recorded vitals.
- A schedule change affects only appointments still in the booked state, and the doctor sees how many appointments it will cancel and confirms first.
- One-off leave (a date range) works like a schedule change.
- A wrongly auto-marked no-show can be reverted by the receptionist or the doctor.
- The out-of-scope list is a proposal.

No open item blocks the next step, which is the tech stack and architecture.
