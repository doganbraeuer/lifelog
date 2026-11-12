# Lifelog

An extensible platform for importing, structuring, storing and searching
personal documents. The first use case is health (findings, doctor's
letters, lab values); further modules such as nutrition, recipes or sport
are planned.

Software practical course, winter term 2026/27, University of Salzburg.

| Folder | Content |
|---|---|
| [`app/`](app/) | Flutter app for iOS and Android |
| [`backend/`](backend/README.md) | Go backend with the REST API |
| [`demo/`](demo/README.md) | Synthetic demo documents and an upload script |
| [`docs/`](docs/) | [API contract](docs/API.md), [glossary](docs/GLOSSARY.md), calibration measurements |

Secrets (API key, database access) are only in `.env`, never in the code;
the template is `.env.example`. All test and demo data is synthetic.
