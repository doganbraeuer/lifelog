import 'api_client.dart';
import 'http_api_client.dart';
import 'mock_api_client.dart';

/// Wert von `--dart-define=API_BASE_URL=...` beim Bauen der App.
/// Leer, wenn nichts angegeben wurde.
const apiBaseUrl = String.fromEnvironment('API_BASE_URL');

/// Ohne `API_BASE_URL` nutzt die App den Mock, sonst das echte Backend.
/// Der Parameter ist nur für Tests da.
ApiClient erstelleApiClient({String baseUrl = apiBaseUrl}) {
  final url = baseUrl.trim();
  return url.isEmpty ? MockApiClient() : HttpApiClient(baseUrl: url);
}
