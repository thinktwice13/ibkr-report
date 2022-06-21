# [WIP] ibkr-report [![goreleaser](https://github.com/thinktwice13/ibkr-report/actions/workflows/release.yaml/badge.svg)](https://github.com/thinktwice13/ibkr-report/actions/workflows/release.yaml)
> A simple Interactive Brokers .csv statements parser intended to help beginner Croatian investors with tax reporting 
## How to use
#### Running the app
Download the latest release and run executable from the root of your directories containing IBKR .csv statements. Find the exported `Portfolio Report.xlsx` spreadsheet in the same directory.

#### Notes
- The statements must be in .csv format
- Duplicate filenames found in subdirectories will be ignored, but make sure there are no extra statements duplicating data (u.e. yearly and monthly statements covering the same period)

If running for the first time, app will also create a `Portfolio Tracker.xlsx` file you can use to add data from other brokers. [See below](#how-to-template)

### Reading the Report
All currencies are in `HRK` (taken from [hnb.hr](https://www.hnb.hr/temeljne-funkcije/monetarna-politika/tecajna-lista/tecajna-lista))

**INO-DOH** sheet matches the same tax report; shows taxed income by source country. Asset countries are determined by reading the provided `ISIN` numbers. If no `ISIN` found, an asset symbol will be used. [See Todos](#todo)

**JOPPD** sheet combines two yearly `JOPPD` reports - realized profits (FIFO method) and dividends. It also considers fees and charges not related to any of the assets (i.e. broker subscription). `JOPPD` report deducts fees from any positive realized profits, but not from the dividend income for the year.

**Summary** sheet shows a breakdown of asset dividends, tax withheld, fees and profits (FIFO method)

**Holdings** sheet lists any active lots still held, their cost in `HRK` and their taxable deadlines (aon 24 months)

### <a name="how-to-template"></a> Using the tracker template
- Show sale trades with negative quantity of shares;
- Show charged fees and withholding tax as negative amounts;
- Use any decimal or 000 separator;
- You can change the order of sheets and columns, but do not change the names;
- For INO-DOH report to work correctly, `ISIN` numbers as asset IDs are preferred over tickers; [See Todos](#todo)
- It is enough to enter `Asset Category` only once per asset across all sheet. Dividends from assets without category found will be included in the tax report [See Todos](#todo)

## <a name="todo"></a> TODO
- Search current holding prices;
- Search asset domicile for INO-DOH report when `ISIN` not provided;
- Search asset categories when not provided;
- Other accounting methods: ACB, LIFO, HIFO, LCFO, LGUT, SLID. For fun;
- Choose directory path for statements;
- Choose report currency, change default to EUR in 2023
- Other brokers


## LICENSE
[MIT](./LICENSE)
