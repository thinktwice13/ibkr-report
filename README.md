# [WIP] Interactive Brokers Report [![test](https://github.com/thinktwice13/ibkr-report/actions/workflows/test.yaml/badge.svg?branch=main)](https://github.com/thinktwice13/ibkr-report/actions/workflows/test.yaml)
> A simple Interactive Brokers .csv statement parser intended to help beginner Croatian investors with tax reporting 
## How to use
#### Running the app
Download the latest release and run executable from the root of your directories containing Interactive Brokers .csv statements. Find the `report.txt` file in the same directory. 

#### Notes
- The statements must be in .csv format
- Duplicate filenames found in subdirectories will be ignored, but make sure there are no extra statements with duplicate data (e.g., yearly and monthly statements both covering the same period)
- The 2023 switch is covered automatically. Years before 2022 are shown in `HRK`, 2023 and later in `EUR`. This cannot be changed.

#### Privacy
- Internet connection is required to fetch currency exchange rates from the Croatian National Bank. No other data is sent or received.

### Todo
- Additional brokers: Revolut, Finax, custom spreadsheet
- See your current holdings and their value. Use unrealized profits and losses to add tax deductions.
- Eliminate need for internet connection by periodically checking and embedding exchange rates in the app