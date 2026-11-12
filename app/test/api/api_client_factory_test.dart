import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/api/api_client_factory.dart';
import 'package:lifelog/api/http_api_client.dart';
import 'package:lifelog/api/mock_api_client.dart';

void main() {
  test('without API_BASE_URL: mock', () {
    expect(createApiClient(baseUrl: ''), isA<MockApiClient>());
    expect(createApiClient(baseUrl: '   '), isA<MockApiClient>());
  });

  test('with API_BASE_URL: HTTP', () {
    final api = createApiClient(baseUrl: 'http://10.0.2.2:8080');
    expect(api, isA<HttpApiClient>());
    expect(api.isMock, isFalse);
    api.close();
  });

  test('API_BASE_URL is not set in the tests', () {
    expect(apiBaseUrl, isEmpty);
  });
}
