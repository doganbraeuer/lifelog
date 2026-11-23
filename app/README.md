# lifelog

A new Flutter project.

## Getting Started

This project is a starting point for a Flutter application.

A few resources to get you started if this is your first Flutter project:

- [Learn Flutter](https://docs.flutter.dev/get-started/learn-flutter)
- [Write your first Flutter app](https://docs.flutter.dev/get-started/codelab)
- [Flutter learning resources](https://docs.flutter.dev/reference/learning-resources)

For help getting started with Flutter development, view the
[online documentation](https://docs.flutter.dev/), which offers tutorials,
samples, guidance on mobile development, and a full API reference.

## Release key („Release-Schlüssel“)

Release APKs are signed with a fixed key, so that a new APK installs over
the old one. `android/app/build.gradle.kts` reads it from
`android/key.properties` at build time. That file and the keystore hold
passwords: they are git-ignored (`key.properties`, `*.jks`, `*.keystore`)
and must never be committed or shared in chat.

`android/key.properties` (placeholders, not real values):

```properties
storePassword=<keystore password>
keyPassword=<key password>
keyAlias=<key alias>
storeFile=<absolute path to the .jks file, outside the repository>
```

Build:

```sh
flutter build apk --release --dart-define=API_BASE_URL=https://lifelog-test.duckdns.org
```

Check the signature (`apksigner` is in the Android SDK under
`build-tools/<version>/`):

```sh
apksigner verify --print-certs build/app/outputs/flutter-apk/app-release.apk
```

The signer must be the release key (`CN=Lifelog Softwarepraktikum`), not
`CN=Android Debug`. If `key.properties` is missing, Gradle prints a warning
and signs with the debug key; do not hand out such an APK.

**Keep the keystore safe.** Android only installs an update if it is signed
with the same key. With another key (or the debug key) testers have to
uninstall the app first, which deletes its local data. An APK that was
signed with the debug key before the release key existed has to be
uninstalled once.
