# govaccine :syringe:

`govaccine` is a small project I developed in just a few hours **to monitor and automatically book appointment slots for COVID-19 shots** (both first shot and booster shot) in France :fr:.
It works with the online appointment platform [Doctolib](https://www.doctolib.fr).

To comply with government rules, **it only looks for "_chronodoses_"** :stopwatch:, i.e. appointment slots which haven't been booked (or have been freed) less than 24 hours prior to the appointment. These _chronodoses_ are **allowed for all people above 18**, unlike other shots.

Only _Pfizer-BioNTech_ and _Moderna_ vaccines are allowed for chronodoses and the algorithm actually **only checks for Pfizer-BioNTech** at this time (Pfizer-BioNTech is the most widespread in Paris vaccination centers currently, I didn't take time to make it work with Moderna).

`govaccine` is much faster :rocket: than browser-based solutions (browser extensions, web automation tools like Selenium, etc.) because it's **100% headless**: there is no interaction with external systems other than Doctolib's API.

The program is **working as of May 26th 2021**, however changes to the Doctolib API or content might break it.

## Usage :open_book:

First, you need to create a text file containing all the Doctolib URLs of the vaccination centers you're interested in, **one URL per line**.
You can find these URLs by searching for vaccination centers on the Doctolib website and copying the URl shown in your browser.
Example:
```text
https://www.doctolib.fr/vaccination-covid-19/paris/centre-de-vaccination-covid-paris-15e
https://www.doctolib.fr/vaccination-covid-19/paris/centre-de-vaccination-cpam-de-paris
https://www.doctolib.fr/vaccination-covid-19/paris/centre-de-vaccination-paris-14e
```
A file is already provided with all Paris vaccination centers in `./assets/paris_vaccination_centers.txt`. You can use it if you want to get vaccinated in Paris. You will need to create your own text file for other places, as explained above.

Then, run the program: `./govaccine -u EMAIL -p PASSWORD -f PATH_TO_YOUR_VACCINATION_CENTERS_TEXTFILE`

The program will exit once an appointment has been booked.

Full usage:
```text
Usage of govaccine:
  -f string
        Filepath of a file containing the URLs of the desired vaccination centers (1 URL per line)
  -p string
        Doctolib password
  -s uint
        Number of seconds between each appointment check for a single worker (default 1)
  -t uint
        Number of seconds after which a request times out (default 5)
  -u string
        Doctolib username (email)
  -w uint
        Number of workers checking for appointments concurrently (default 4)
```

## Personal data :memo:

The program only communicates with `doctolib.fr` in HTTPS (HTTP Secure). No data is sent anywhere else and no data is stored by the program.

## Technical details :desktop_computer:

As its name suggests, `govaccine` is written in Go... solely because I wanted to practice Go (so the code might not be very idiomatic).

To compile the program, go to the `./cmd/govaccine/` directory and execute `go build .` This will create the `govaccine` executable file which you can run as explained above.

## Known issues :bug:

### Unconfirmed appointment

`govaccine` assumes the appointment is confirmed and exits as long as it issued the _request_ to confirm the appointment (without actually checking the _response_ of the request).

If someone else is booking the same appointment and confirms it first, this will cause `govaccine` to believe you got the appointment and exit (while in fact the appointment was given to someone else).

**The workaround is simply to re-run `govaccine` to book another appointment.**

This issue hasn't been fixed yet because one needs to know the response returned by the API in the case described above, which is not easy to reproduce.
