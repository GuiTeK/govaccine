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
package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"github.com/GuiTeK/govaccine/internal/app/govaccine"
	"github.com/GuiTeK/govaccine/internal/pkg/utils"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

func getVaccinationCenters(vaccinationCentersFilepath string) ([]string, error) {
	file, err := os.Open(vaccinationCentersFilepath)
	if err != nil {
		return nil, fmt.Errorf("main.getVaccinationCenters(): failed to open file %s: %s",
			vaccinationCentersFilepath, err)
	}
	defer func() {
		_ = file.Close()
	}()

	var vaccinationCenters []string
	reader := bufio.NewReader(file)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				return nil, fmt.Errorf(
					"main.getVaccinationCenters(): encountered error while reading file %s: %s",
					vaccinationCentersFilepath, err)
			}

			break
		}

		line = strings.Replace(line, "https://", "", -1)
		line = strings.Replace(line, "http://", "", -1)
		line = strings.Replace(line, "www.doctolib.fr/", "", -1)
		line = strings.Replace(line, "doctolib.fr/", "", -1)
		line = strings.Split(line, "?")[0]
		urlParts := strings.Split(line, "/")

		if len(urlParts) == 3 {
			vaccinationCenterName := strings.Replace(strings.Replace(urlParts[2], "\r", "", -1),
				"\n", "", -1)
			vaccinationCenters = append(vaccinationCenters, vaccinationCenterName)
		}
	}

	if len(vaccinationCenters) == 0 {
		return nil, fmt.Errorf("main.getVaccinationCenters(): no vaccination center URL found in file %s",
			vaccinationCentersFilepath)
	}

	return vaccinationCenters, nil
}

func parseArgs(doctolibUsername *string, doctolibPassword *string, vaccinationCentersFilepath *string,
	workersNb *uint, sleepTime *uint, requestsTimeout *uint) error {
	flag.StringVar(doctolibUsername, "u", "", "Doctolib username (email)")
	flag.StringVar(doctolibPassword, "p", "", "Doctolib password")
	flag.StringVar(vaccinationCentersFilepath, "f", "",
		"Filepath of a file containing the URLs of the desired vaccination centers (1 URL per line)")
	flag.UintVar(workersNb, "w", 4, "Number of workers checking for appointments concurrently")
	flag.UintVar(sleepTime, "s", 1,
		"Number of seconds between each appointment check for a single worker")
	flag.UintVar(requestsTimeout, "t", 5, "Number of seconds after which a request times out")

	flag.Parse()

	if *doctolibUsername == "" {
		return errors.New("Doctolib username (-u flag) is required")
	}

	if *doctolibPassword == "" {
		return errors.New("Doctolib password (-p flag) is required")
	}

	if *vaccinationCentersFilepath == "" {
		return errors.New("Vaccination centers filepath (-f flag) is required")
	}

	if *workersNb == 0 || *workersNb > 16 {
		return errors.New("number of workers should be >= 0 and <= 16")
	}

	return nil
}

func main() {
	var doctolibUsername string
	var doctolibPassword string
	var vaccinationCentersFilepath string
	var workersNb uint
	var sleepTime uint
	var requestsTimeout uint

	if err := parseArgs(&doctolibUsername, &doctolibPassword, &vaccinationCentersFilepath, &workersNb, &sleepTime,
		&requestsTimeout); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "%s\n", err)
		flag.Usage()
		os.Exit(1)
	}

	vaccinationCenters, err := getVaccinationCenters(vaccinationCentersFilepath)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "[ERROR] failed to read vaccination centers: %s\n", err)
		os.Exit(1)
	}

	sleepTimeDuration := time.Duration(sleepTime) * time.Second
	requestsTimeoutDuration := time.Duration(requestsTimeout) * time.Second
	stop := make(chan bool)
	mutex := &sync.Mutex{}
	jobs := make(chan string, workersNb)
	waitGroup := &sync.WaitGroup{}
	for i := uint(0); i < workersNb; i++ {
		botName := fmt.Sprintf("Worker %d", i+1)
		vaccibot, err := govaccine.NewVaccibot(botName, doctolibUsername, doctolibPassword, jobs, stop, mutex,
			sleepTimeDuration, requestsTimeoutDuration)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "[ERROR] failed to create Vaccibot \"%s\": %s\n", botName, err)
			os.Exit(1)
		}

		waitGroup.Add(1)
		go func(v *govaccine.Vaccibot) {
			defer waitGroup.Done()
			vaccibot.TryBookVaccine()
		}(vaccibot)
	}

	i := 0
	for {
		if utils.IsBoolChannelClosed(stop) {
			fmt.Printf("[INFO] Vaccibot orchestrator received stop signal\n")
			close(jobs)
			break
		}

		if i == len(vaccinationCenters) {
			i = 0
		}
		jobs <- vaccinationCenters[i]

		i = i + 1
	}

	fmt.Println("[INFO] Shutting down...")
	waitGroup.Wait()
}
