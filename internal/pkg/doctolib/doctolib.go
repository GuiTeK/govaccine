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
package doctolib

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	url2 "net/url"
	"strings"
	"time"
)

type Client struct {
	httpClient *http.Client
}

type loginPayload struct {
	Remember         bool   `json:"remember"`
	RememberUsername bool   `json:"remember_username"`
	Username         string `json:"username"`
	Password         string `json:"password"`
	Kind             string `json:"kind"`
}

type LoginResponse struct {
	Id        int    `json:"id"`
	FullName  string `json:"full_name"`
	CsrfToken string
}

type BookingProfile struct {
	Id int `json:"id"`
}

type BookingVisitMotive struct {
	Id   int    `json:"id"`
	Name string `json:"name"`
}

type BookingAgenda struct {
	Id                       int   `json:"id"`
	BookingDisabled          bool  `json:"booking_disabled"`
	BookingTemporaryDisabled bool  `json:"booking_temporary_disabled"`
	VisitMotiveIds           []int `json:"visit_motive_ids"`
	PracticeId               int   `json:"practice_id"`
}

type BookingResponseData struct {
	Profile      BookingProfile       `json:"profile"`
	VisitMotives []BookingVisitMotive `json:"visit_motives"`
	Agendas      []BookingAgenda      `json:"agendas"`
}

type BookingResponse struct {
	Data      BookingResponseData `json:"data"`
	CsrfToken string
}

type availabilitySlotStep struct {
	StartDate string `json:"start_date"`
}

type availabilitySlot struct {
	StartDate string                 `json:"start_date"`
	Steps     []availabilitySlotStep `json:"steps"`
}

type availability struct {
	Date  string             `json:"date"`
	Slots []availabilitySlot `json:"slots"`
}

type AvailabilitiesResponse struct {
	Availabilities []availability `json:"availabilities"`
	Total          int            `json:"total"`
	CsrfToken      string
}

type appointmentPayload struct {
	StartDate      string `json:"start_date"`
	VisitMotiveIds string `json:"visit_motive_ids"`
	ProfileId      int    `json:"profile_id"`
	SourceAction   string `json:"source_action"`
}

type createAppointmentPayload struct {
	AgendaIds   string             `json:"agenda_ids"`
	PracticeIds []int              `json:"practice_ids"`
	Appointment appointmentPayload `json:"appointment"`
}

type createAppointmentSecondPayload struct {
	AgendaIds   string             `json:"agenda_ids"`
	PracticeIds []int              `json:"practice_ids"`
	Appointment appointmentPayload `json:"appointment"`
	SecondSlot  string             `json:"second_slot"`
}

type CreateAppointmentResponse struct {
	Id        string `json:"id"`
	CsrfToken string
}

type MasterPatient struct {
	Id                int    `json:"id"`
	FirstName         string `json:"first_name"`
	LastName          string `json:"last_name"`
	Kind              string `json:"kind"`
	Gender            bool   `json:"gender"`
	Birthdate         string `json:"birthdate"`
	IsComplete        bool   `json:"is_complete"`
	HasOwnEmail       bool   `json:"has_own_email"`
	HasOwnPhoneNumber bool   `json:"has_own_phone_number"`
	Email             string `json:"email"`
	PhoneNumber       string `json:"phone_number"`
	MismatchInsurance bool   `json:"mismatchInsurance"`
	Consented         bool   `json:"consented"`
}

type MasterPatientsResponse struct {
	MasterPatients []MasterPatient
	CsrfToken      string
}

type confirmedAppointment struct {
	QualificationAnswers map[string]string `json:"qualification_answers"`
	NewPatient           bool              `json:"new_patient"`
	StartDate            string            `json:"start_date"`
	CustomFieldsValues   map[string]string `json:"custom_fields_values"`
	ReferrerId           *int              `json:"referrer_id"`
}

type confirmAppointmentPayload struct {
	NewPatient                         bool                 `json:"new_patient"`
	BypassMandatoryRelativeContactInfo bool                 `json:"bypass_mandatory_relative_contact_info"`
	PhoneNumber                        *string              `json:"phone_number"`
	Email                              *string              `json:"email"`
	MasterPatient                      MasterPatient        `json:"master_patient"`
	Patient                            *MasterPatient       `json:"patient"`
	Appointment                        confirmedAppointment `json:"appointment"`
}

type ConfirmAppointmentResponse struct {
	CsrfToken string
}

const RootUrl = "https://doctolib.fr"

func addCommonHeaders(req *http.Request, isFetchJson bool, csrfToken string) {
	if isFetchJson {
		req.Header.Set("accept", "application/json")
		req.Header.Set("content-type", "application/json; charset=utf-8")
	} else {
		req.Header.Set("accept",
			"text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.9")
	}

	req.Header.Set("user-agent",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/90.0.4430.212 Safari/537.36")

	if csrfToken != "" {
		req.Header.Set("x-csrf-token", csrfToken)
	}
}

func (c *Client) ConfirmAppointment(appointmentId string, startDatetime string, masterPatient MasterPatient,
	csrfToken string) (*ConfirmAppointmentResponse, error) {
	url := fmt.Sprintf("%s/appointments/%s.json", RootUrl, appointmentId)

	var payloadBytes []byte
	var err error

	payload := confirmAppointmentPayload{
		NewPatient:                         true,
		BypassMandatoryRelativeContactInfo: false,
		PhoneNumber:                        nil,
		Email:                              nil,
		MasterPatient:                      masterPatient,
		Patient:                            nil,
		Appointment: confirmedAppointment{
			QualificationAnswers: make(map[string]string),
			NewPatient:           true,
			StartDate:            startDatetime,
			CustomFieldsValues:   make(map[string]string),
			ReferrerId:           nil,
		},
	}
	payloadBytes, err = json.Marshal(payload)

	if err != nil {
		return nil, fmt.Errorf("doctolib.ConfirmAppointment(): cannot marshal login payload: %w", err)
	}

	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("doctolib.ConfirmAppointment(): cannot create request %s: %w", url, err)
	}

	addCommonHeaders(req, true, csrfToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("doctolib.ConfirmAppointment(): cannot do request %s: %w", url, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("doctolib.ConfirmAppointment(): unexpected response status code (%d) for %s",
			resp.StatusCode, url)
	}

	responseBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("doctolib.ConfirmAppointment(): cannot read response of request %s: %w", url, err)
	}

	var response ConfirmAppointmentResponse
	err = json.Unmarshal(responseBytes, &response)
	if err != nil {
		return nil, fmt.Errorf("doctolib.ConfirmAppointment(): cannot unmarshal response of request %s: %w",
			url, err)
	}

	response.CsrfToken = resp.Header.Get("x-csrf-token")
	if response.CsrfToken == "" {
		return nil, fmt.Errorf("doctolib.ConfirmAppointment(): no CSRF token found in response")
	}

	return &response, nil
}

func (c *Client) GetMasterPatients(csrfToken string) (*MasterPatientsResponse, error) {
	url := fmt.Sprintf("%s/account/master_patients.json", RootUrl)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("doctolib.GetMasterPatients(): cannot create request %s: %w", url, err)
	}

	addCommonHeaders(req, true, csrfToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("doctolib.GetMasterPatients(): cannot do request %s: %w", url, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("doctolib.GetMasterPatients(): unexpected response status code (%d) for %s",
			resp.StatusCode, url)
	}

	responseBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("doctolib.GetMasterPatients(): cannot read response of request %s: %w", url, err)
	}

	var masterPatients []MasterPatient
	err = json.Unmarshal(responseBytes, &masterPatients)
	if err != nil {
		return nil, fmt.Errorf("doctolib.GetMasterPatients(): cannot unmarshal response of request %s: %w",
			url, err)
	}

	for _, masterPatient := range masterPatients {
		masterPatient.MismatchInsurance = false
		masterPatient.Consented = true
	}

	response := MasterPatientsResponse{
		MasterPatients: masterPatients,
		CsrfToken:      resp.Header.Get("x-csrf-token"),
	}
	if response.CsrfToken == "" {
		return nil, fmt.Errorf("doctolib.GetMasterPatients(): no CSRF token found in response")
	}

	return &response, nil
}

func (c *Client) CreateAppointment(startDatetime string, secondSlotDatetime string, visitMotiveIds []int,
	agendaIds []int, practiceIds []int, profileId int, csrfToken string) (*CreateAppointmentResponse, error) {
	url := fmt.Sprintf("%s/appointments.json", RootUrl)

	formattedAgendaIds := strings.Trim(strings.Join(strings.Split(fmt.Sprint(agendaIds), " "), "-"),
		"[]")
	formattedVisitMotiveIds := strings.Trim(strings.Join(strings.Split(fmt.Sprint(visitMotiveIds), " "), "-"),
		"[]")
	var payloadBytes []byte
	var err error

	if secondSlotDatetime == "" {
		payload := createAppointmentPayload{
			AgendaIds:   formattedAgendaIds,
			PracticeIds: practiceIds,
			Appointment: appointmentPayload{
				StartDate:      startDatetime,
				VisitMotiveIds: formattedVisitMotiveIds,
				ProfileId:      profileId,
				SourceAction:   "profile",
			},
		}
		payloadBytes, err = json.Marshal(payload)
	} else {
		payload := createAppointmentSecondPayload{
			AgendaIds:   formattedAgendaIds,
			PracticeIds: practiceIds,
			Appointment: appointmentPayload{
				StartDate:      startDatetime,
				VisitMotiveIds: formattedVisitMotiveIds,
				ProfileId:      profileId,
				SourceAction:   "profile",
			},
			SecondSlot: secondSlotDatetime,
		}
		payloadBytes, err = json.Marshal(payload)
	}

	if err != nil {
		return nil, fmt.Errorf("doctolib.CreateAppointment(): cannot marshal login payload: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("doctolib.CreateAppointment(): cannot create request %s: %w", url, err)
	}

	addCommonHeaders(req, true, csrfToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("doctolib.CreateAppointment(): cannot do request %s: %w", url, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("doctolib.CreateAppointment(): unexpected response status code (%d) for %s",
			resp.StatusCode, url)
	}

	responseBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("doctolib.CreateAppointment(): cannot read response of request %s: %w", url, err)
	}

	var response CreateAppointmentResponse
	err = json.Unmarshal(responseBytes, &response)
	if err != nil {
		return nil, fmt.Errorf("doctolib.CreateAppointment(): cannot unmarshal response of request %s: %w",
			url, err)
	}

	if response.Id == "" {
		return nil, fmt.Errorf("doctolib.CreateAppointment(): no appointment ID in response of request %s: %s",
			url, string(responseBytes))
	}

	response.CsrfToken = resp.Header.Get("x-csrf-token")
	if response.CsrfToken == "" {
		return nil, fmt.Errorf("doctolib.CreateAppointment(): no CSRF token found in response")
	}

	return &response, nil
}

func (c *Client) GetAvailabilities(startDate time.Time, firstSlotDatetime *time.Time, visitMotiveIds []int,
	agendaIds []int, practiceIds []int, limit int, csrfToken string) (*AvailabilitiesResponse, error) {
	url := fmt.Sprintf("%s/availabilities.json", RootUrl)

	if firstSlotDatetime != nil {
		url = fmt.Sprintf("%s/second_shot_availabilities.json", RootUrl)
	}

	formattedStartDate := startDate.Format("2006-01-02")
	formattedVisitMotiveIds := strings.Trim(strings.Join(strings.Split(fmt.Sprint(visitMotiveIds), " "), "-"),
		"[]")
	formattedAgendaIds := strings.Trim(strings.Join(strings.Split(fmt.Sprint(agendaIds), " "), "-"),
		"[]")
	formattedPracticeIds := strings.Trim(strings.Join(strings.Split(fmt.Sprint(practiceIds), " "), "-"),
		"[]")
	url = fmt.Sprintf(
		"%s?start_date=%s&limit=%d&visit_motive_ids=%s&agenda_ids=%s&practice_ids=%s&insurance_sector=public",
		url, formattedStartDate, limit, formattedVisitMotiveIds, formattedAgendaIds, formattedPracticeIds)

	if firstSlotDatetime != nil {
		formattedFirstSlot := url2.QueryEscape(firstSlotDatetime.Format("2006-01-02T15:04:05.000-07:00"))
		url = fmt.Sprintf("%s&first_slot=%s", url, formattedFirstSlot)
	} else {
		url = fmt.Sprintf("%s&destroy_temporary=true", url) // Destroys any appointment not yet confirmed
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("doctolib.GetAvailabilities(): cannot create request %s: %w", url, err)
	}

	addCommonHeaders(req, true, csrfToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("doctolib.GetAvailabilities(): cannot do request %s: %w", url, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("doctolib.GetAvailabilities(): unexpected response status code (%d) for %s",
			resp.StatusCode, url)
	}

	responseBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("doctolib.GetAvailabilities(): cannot read response of request %s: %w", url, err)
	}

	var response AvailabilitiesResponse
	err = json.Unmarshal(responseBytes, &response)
	if err != nil {
		return nil, fmt.Errorf("doctolib.GetAvailabilities(): cannot unmarshal response of request %s: %w",
			url, err)
	}

	response.CsrfToken = resp.Header.Get("x-csrf-token")
	if response.CsrfToken == "" {
		return nil, fmt.Errorf("doctolib.GetAvailabilities(): no CSRF token found in response")
	}

	return &response, nil
}

func (c *Client) GetBooking(placeName string, csrfToken string) (*BookingResponse, error) {
	url := fmt.Sprintf("%s/booking/%s.json", RootUrl, placeName)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("doctolib.GetBooking(): cannot create request %s: %w", url, err)
	}

	addCommonHeaders(req, true, csrfToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("doctolib.GetBooking(): cannot do request %s: %w", url, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("doctolib.GetBooking(): unexpected response status code (%d) for %s",
			resp.StatusCode, url)
	}

	responseBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("doctolib.GetBooking(): cannot read response of request %s: %w", url, err)
	}

	var response BookingResponse
	err = json.Unmarshal(responseBytes, &response)
	if err != nil {
		return nil, fmt.Errorf("doctolib.GetBooking(): cannot unmarshal response of request %s: %w", url, err)
	}

	response.CsrfToken = resp.Header.Get("x-csrf-token")
	if response.CsrfToken == "" {
		return nil, fmt.Errorf("doctolib.GetBooking(): no CSRF token found in response")
	}

	return &response, nil
}

func (c *Client) getInitialCsrfToken() (string, error) {
	sessionsNewUrl := fmt.Sprintf("%s/sessions/new", RootUrl)

	req, err := http.NewRequest("GET", sessionsNewUrl, nil)
	if err != nil {
		return "", fmt.Errorf("doctolib.getInitialCsrfToken(): cannot create request %s: %w",
			sessionsNewUrl, err)
	}

	addCommonHeaders(req, false, "")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("doctolib.getInitialCsrfToken(): cannot do request %s: %w", sessionsNewUrl, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("doctolib.getInitialCsrfToken(): unexpected response status code (%d) for %s",
			resp.StatusCode, sessionsNewUrl)
	}

	csrfToken := resp.Header.Get("x-csrf-token")
	if csrfToken == "" {
		return "", fmt.Errorf("doctolib.getInitialCsrfToken(): no CSRF token found in response")
	}

	return csrfToken, nil
}

func (c *Client) Login(username string, password string) (*LoginResponse, error) {
	csrfToken, err := c.getInitialCsrfToken()
	if err != nil {
		return nil, fmt.Errorf("doctolib.Login(): cannot get CSRF token for login: %w", err)
	}

	url := fmt.Sprintf("%s/login.json", RootUrl)
	payload := loginPayload{
		Remember:         true,
		RememberUsername: true,
		Username:         username,
		Password:         password,
		Kind:             "patient",
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("doctolib.Login(): cannot marshal login payload: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("doctolib.Login(): cannot create request %s: %w", url, err)
	}

	addCommonHeaders(req, true, csrfToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("doctolib.Login(): cannot do request %s: %w", url, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("doctolib.Login(): unexpected response status code (%d) for %s",
			resp.StatusCode, url)
	}

	responseBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("doctolib.Login(): cannot read response of request %s: %w", url, err)
	}

	var response LoginResponse
	err = json.Unmarshal(responseBytes, &response)
	if err != nil {
		return nil, fmt.Errorf("doctolib.Login(): cannot unmarshal response of request %s: %w", url, err)
	}

	response.CsrfToken = resp.Header.Get("x-csrf-token")
	if response.CsrfToken == "" {
		return nil, fmt.Errorf("doctolib.Login(): no CSRF token found in response")
	}

	return &response, nil
}

func NewClient(requestsTimeout time.Duration) (*Client, error) {
	cookieJar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("doctolib.NewClient(): cannot create cookie jar: %w", err)
	}

	doctolibClient := &Client{}
	doctolibClient.httpClient = &http.Client{
		Transport:     nil,
		CheckRedirect: nil,
		Jar:           cookieJar,
		Timeout:       requestsTimeout,
	}

	return doctolibClient, nil
}
