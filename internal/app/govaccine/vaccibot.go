/*
 * MIT License
 *
 * Copyright (c) 2021 Guillaume Truchot
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this software and associated documentation files (the "Software"), to deal
 * in the Software without restriction, including without limitation the rights
 * to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is
 * furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in all
 * copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
 * SOFTWARE.
 */
package govaccine

import (
	"fmt"
	"github.com/GuiTeK/govaccine/internal/pkg/doctolib"
	"github.com/GuiTeK/govaccine/internal/pkg/utils"
	"sync"
	"time"
)

type Vaccibot struct {
	name             string
	jobs             chan string
	stop             chan bool
	mutex            *sync.Mutex
	doctolibClient   *doctolib.Client
	sleepDuration    time.Duration
	currentCsrfToken string
}

type vaccinationSettings struct {
	profileId      int
	visitMotiveIds []int
	agendaIds      []int
	practiceIds    []int
	csrfToken      string
}

const PfizerBiontechVaccineVisitMotiveName = "1re injection vaccin COVID-19 (Pfizer-BioNTech)"

func (v *Vaccibot) getVaccinationSettings(vaccinationCenter string, csrfToken string) (*vaccinationSettings, error) {
	bookingResponse, err := v.doctolibClient.GetBooking(vaccinationCenter, csrfToken)
	if err != nil {
		return nil, fmt.Errorf("govaccine.getVaccinationSettings(): failed to get booking for %s: %s",
			vaccinationCenter, err)
	}

	vacSettings := &vaccinationSettings{
		profileId: bookingResponse.Data.Profile.Id,
	}
	for _, visitMotive := range bookingResponse.Data.VisitMotives {
		if visitMotive.Name != PfizerBiontechVaccineVisitMotiveName {
			continue
		}

		if len(vacSettings.visitMotiveIds) > 0 {
			return nil, fmt.Errorf(
				"govaccine.getVaccinationSettings(): unhandled case: vaccination center %s has multiple choices for Pfizer-BioNTech 1st injection",
				vaccinationCenter)
		}

		vacSettings.visitMotiveIds = append(vacSettings.visitMotiveIds, visitMotive.Id)
	}

	if len(vacSettings.visitMotiveIds) == 0 {
		return nil, fmt.Errorf(
			"govaccine.getVaccinationSettings(): cannot find any visit motive ID for vaccination center %s",
			vaccinationCenter)
	}

	for _, agenda := range bookingResponse.Data.Agendas {
		if !utils.IntSliceContains(agenda.VisitMotiveIds, vacSettings.visitMotiveIds[0]) {
			continue
		}

		if agenda.BookingDisabled || agenda.BookingTemporaryDisabled {
			fmt.Printf(
				"govaccine.getVaccinationSettings(): warning: agenda %d is disabled for vaccination center %s\n",
				agenda.Id, vaccinationCenter)
			continue
		}

		vacSettings.agendaIds = append(vacSettings.agendaIds, agenda.Id)

		if !utils.IntSliceContains(vacSettings.practiceIds, agenda.PracticeId) {
			vacSettings.practiceIds = append(vacSettings.practiceIds, agenda.PracticeId)
		}
	}

	if len(vacSettings.agendaIds) == 0 {
		return nil, fmt.Errorf(
			"govaccine.getVaccinationSettings(): cannot find any agenda/practice IDs for vaccination center %s",
			vaccinationCenter)
	}

	vacSettings.csrfToken = bookingResponse.CsrfToken

	return vacSettings, nil
}

func (v *Vaccibot) TryBookVaccine() {
	for vaccinationCenter := range v.jobs {
		fmt.Printf("[INFO] Vaccibot \"%s\" is checking %s\n", v.name, vaccinationCenter)

		if utils.IsBoolChannelClosed(v.stop) {
			fmt.Printf("[INFO] Vaccibot \"%s\" received stop signal\n", v.name)
			return
		}
		time.Sleep(v.sleepDuration)

		vaccinationSettings, err := v.getVaccinationSettings(vaccinationCenter, v.currentCsrfToken)
		if err != nil {
			fmt.Printf("[WARNING] Vaccibot \"%s\" failed to get vaccination settings: %s\n", v.name, err)
			continue
		}
		v.currentCsrfToken = vaccinationSettings.csrfToken

		startDate := time.Now().AddDate(0, 0, 1)
		firstShotAvailabilitiesResponse, err := v.doctolibClient.GetAvailabilities(startDate, nil,
			vaccinationSettings.visitMotiveIds, vaccinationSettings.agendaIds, vaccinationSettings.practiceIds,
			1, v.currentCsrfToken)
		if err != nil {
			fmt.Printf("[ERROR] Vaccibot \"%s\" failed to get first shot availabilities: %s\n", v.name, err)
			continue
		}
		v.currentCsrfToken = firstShotAvailabilitiesResponse.CsrfToken
		if firstShotAvailabilitiesResponse.Total == 0 {
			continue // No availability for now
		}

		v.mutex.Lock() // Prevent two appointment bookings at the same time

		// Make sure no appointment was booked by another worker while we were waiting to acquire the lock
		if utils.IsBoolChannelClosed(v.stop) {
			fmt.Printf("[INFO] Vaccibot \"%s\" received stop signal\n", v.name)
			v.mutex.Unlock()
			return
		}

		createFirstShotAppointmentResponse, err := v.doctolibClient.CreateAppointment(
			firstShotAvailabilitiesResponse.Availabilities[0].Slots[0].StartDate, "",
			vaccinationSettings.visitMotiveIds, vaccinationSettings.agendaIds, vaccinationSettings.practiceIds,
			vaccinationSettings.profileId, v.currentCsrfToken)
		if err != nil {
			fmt.Printf("[ERROR] Vaccibot \"%s\" failed to create first shot appointment: %s\n", v.name, err)
			v.mutex.Unlock()
			continue
		}
		v.currentCsrfToken = createFirstShotAppointmentResponse.CsrfToken
		fmt.Printf("[INFO] Vaccibot \"%s\" created first shot appointment (ID %s)\n",
			v.name, createFirstShotAppointmentResponse.Id)

		secondShotStartDatetime, err := time.Parse("2006-01-02T15:04:05.000-07:00",
			firstShotAvailabilitiesResponse.Availabilities[0].Slots[0].Steps[1].StartDate)
		if err != nil {
			fmt.Printf(
				"[ERROR] Vaccibot \"%s\" failed to parse second shot start datetime (%s): %s\n",
				v.name, firstShotAvailabilitiesResponse.Availabilities[0].Slots[0].Steps[1].StartDate, err)
			v.mutex.Unlock()
			continue
		}
		firstShotDatetime, err := time.Parse("2006-01-02T15:04:05.000-07:00",
			firstShotAvailabilitiesResponse.Availabilities[0].Slots[0].StartDate)
		if err != nil {
			fmt.Printf("[ERROR] Vaccibot \"%s\" failed to parse first shot datetime (%s): %s\n",
				v.name, firstShotAvailabilitiesResponse.Availabilities[0].Slots[0].StartDate, err)
			v.mutex.Unlock()
			continue
		}
		secondShotAvailabilitiesResponse, err := v.doctolibClient.GetAvailabilities(secondShotStartDatetime,
			&firstShotDatetime,
			vaccinationSettings.visitMotiveIds, vaccinationSettings.agendaIds, vaccinationSettings.practiceIds,
			4, v.currentCsrfToken)
		if err != nil {
			fmt.Printf("[ERROR] Vaccibot \"%s\" failed to get second shot availabilities: %s\n", v.name, err)
			v.mutex.Unlock()
			continue
		}
		v.currentCsrfToken = secondShotAvailabilitiesResponse.CsrfToken
		if secondShotAvailabilitiesResponse.Total == 0 {
			fmt.Printf(
				"[INFO] Vaccibot \"%s\" second shot no more available for appointment (ID %s)\n",
				v.name, createFirstShotAppointmentResponse.Id)
			v.mutex.Unlock()
			continue
		}

		createSecondShotAppointmentResponse, err := v.doctolibClient.CreateAppointment(
			firstShotAvailabilitiesResponse.Availabilities[0].Slots[0].StartDate,
			secondShotAvailabilitiesResponse.Availabilities[0].Slots[0].StartDate,
			vaccinationSettings.visitMotiveIds, vaccinationSettings.agendaIds, vaccinationSettings.practiceIds,
			vaccinationSettings.profileId, v.currentCsrfToken)
		if err != nil {
			fmt.Printf(
				"[ERROR] Vaccibot \"%s\" failed to create second shot appointment (ID %s): %s\n",
				v.name, createFirstShotAppointmentResponse.Id, err)
			v.mutex.Unlock()
			continue
		}
		v.currentCsrfToken = createSecondShotAppointmentResponse.CsrfToken
		fmt.Printf("[INFO] Vaccibot \"%s\" created second shot appointment (ID %s)\n",
			v.name, createSecondShotAppointmentResponse.Id)

		masterPatientsResponse, err := v.doctolibClient.GetMasterPatients(v.currentCsrfToken)
		if err != nil {
			fmt.Printf("[ERROR] Vaccibot \"%s\" failed to get master patients: %s\n", v.name, err)
			v.mutex.Unlock()
			continue
		}
		v.currentCsrfToken = masterPatientsResponse.CsrfToken

		_, err = v.doctolibClient.ConfirmAppointment(createFirstShotAppointmentResponse.Id,
			firstShotAvailabilitiesResponse.Availabilities[0].Slots[0].StartDate,
			masterPatientsResponse.MasterPatients[0], v.currentCsrfToken)
		if err != nil {
			fmt.Printf("[ERROR] Vaccibot \"%s\" failed to confirm appointment (ID %s): %s\n",
				v.name, createSecondShotAppointmentResponse.Id, err)
			v.mutex.Unlock()
			continue
		}
		fmt.Printf("[INFO] Vaccibot \"%s\" successfully confirmed the appointment, congratulations!\n", v.name)
		close(v.stop)
		v.mutex.Unlock()
	}
}

func NewVaccibot(name string, doctolibUsername string, doctolibPassword string, jobs chan string, stop chan bool,
	mutex *sync.Mutex, sleepDuration time.Duration, requestsTimeout time.Duration) (*Vaccibot, error) {
	doctolibClient, err := doctolib.NewClient(requestsTimeout)
	if err != nil {
		return nil, fmt.Errorf("govaccine.NewVaccibot(): cannot create Doctolib client: %w", err)
	}

	vaccibot := &Vaccibot{
		name:           name,
		jobs:           jobs,
		stop:           stop,
		mutex:          mutex,
		doctolibClient: doctolibClient,
		sleepDuration:  sleepDuration,
	}

	loginResponse, err := vaccibot.doctolibClient.Login(doctolibUsername, doctolibPassword)
	if err != nil {
		return nil, fmt.Errorf("govaccine.NewVaccibot(): failed to login: %w", err)
	}
	fmt.Printf("[INFO] Vaccibot \"%s\" logged in as %s (ID %d)\n", name, loginResponse.FullName, loginResponse.Id)

	vaccibot.currentCsrfToken = loginResponse.CsrfToken

	return vaccibot, nil
}
