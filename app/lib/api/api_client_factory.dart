import 'api_client.dart';
import 'http_api_client.dart';
import 'mock_api_client.dart';

/// Value of `--dart-define=API_BASE_URL=...` when building the app.
/// Empty if nothing was given.
const apiBaseUrl = String.fromEnvironment('API_BASE_URL');

/// Without `API_BASE_URL` the app uses the mock, otherwise the real backend.
/// The parameter only exists for tests.
ApiClient createApiClient({String baseUrl = apiBaseUrl}) {
  final url = baseUrl.trim();
  return url.isEmpty ? MockApiClient() : HttpApiClient(baseUrl: url);
}
