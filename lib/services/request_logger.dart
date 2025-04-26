import 'package:dio/dio.dart';

class RequestLogger {
  static final List<String> _logs = [];

  static void addLog(RequestOptions options) {
    final buffer = StringBuffer();
    buffer.writeln('URL: ${options.uri}');
    buffer.writeln('METHOD: ${options.method}');
    buffer.writeln('HEADERS: ${options.headers}');
    if (options.data != null) {
      buffer.writeln('BODY: ${options.data}');
    }
    _logs.add(buffer.toString());
  }

  static List<String> get logs => _logs;
}
