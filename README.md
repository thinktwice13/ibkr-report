# [WIP] Interactive Brokers Report [![test](https://github.com/thinktwice13/ibkr-report/actions/workflows/test.yaml/badge.svg?branch=main)](https://github.com/thinktwice13/ibkr-report/actions/workflows/test.yaml)
> A simple Interactive Brokers .csv statement parser intended to help beginner Croatian investors with tax reporting 
## How to use
#### Running the app
Download the latest release and run executable from the root of your directories containing Interactive Brokers .csv statements. Find the `report.txt` file in the same directory. \
The app does NOT calculate tax payments due to different surtax rates by location. To be implemented. 

#### Notes
- The statements must be in .csv format
- Duplicate filenames found in subdirectories will be ignored, but make sure there are no extra statements with duplicate data (e.g., yearly and monthly statements both covering the same period)
- The 2023 switch is covered automatically. Years before 2022 are shown in `HRK`, 2023 and later in `EUR`. This cannot be changed.

#### Report example
- **Valuta** is the currency of the report for the given year
- **Izvješće** is the type of report
- **Dobit** is the profit in the given currency
- **Izvor prihoda** is the source of income reported only for `INO-DOH` reports
- **Plaćeni porez** is the tax paid in the given currency at the source listed in the INO-DOH report
```
Godina  Valuta  Izvješće    Dobit    Izvor prihoda  Plaćeni porez   
2021    HRK     JOPPD       10000.99                                 
2021    HRK     INO-DOH     5000.00  US             1500.00          
2022    HRK     JOPPD       0.00                                   
2022    HRK     INO-DOH     500.00   US             150.00
2022    HRK     INO-DOH     10000.00 AT             1500.00
2023    EUR     JOPPD       1000.00                                   
2023    EUR     INO-DOH     1000.00   US            300.00           
```

#### Privacy
- Internet connection is required to fetch currency exchange rates from the Croatian National Bank. No other data is sent or received.

### Todo
- Additional brokers: Revolut, Finax, custom spreadsheet
- See your current holdings and their value. Use unrealized profits and losses to add tax deductions.
- Eliminate need for internet connection by periodically checking and embedding exchange rates in the app
- Implement automatic tax payment calculation (issue)