import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/api/api_client_factory.dart';
import 'package:lifelog/api/http_api_client.dart';
import 'package:lifelog/api/mock_api_client.dart';

void main() {
  test('ohne API_BASE_URL: Mock', () {
    expect(createApiClient(baseUrl: ''), isA<MockApiClient>());
    expect(createApiClient(baseUrl: '   '), isA<MockApiClient>());
  });

  test('mit API_BASE_URL: HTTP', () {
    final api = createApiClient(baseUrl: 'http://10.0.2.2:8080');
    expect(api, isA<HttpApiClient>());
    expect(api.isMock, isFalse);
    api.close();
  });

  test('in den Tests ist API_BASE_URL nicht gesetzt', () {
    expect(apiBaseUrl, isEmpty);
  });
}
