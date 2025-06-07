import 'package:dio/dio.dart';
import 'package:flutter_dotenv/flutter_dotenv.dart';

class DoctorRoomService {
  final String _baseUrl = dotenv.env['API_BASE_URL']!;
  final Dio _dio;

  DoctorRoomService(this._dio);

  /// Получить список всех кабинетов
  Future<List<dynamic>> getDoctorRooms() async {
    try {
      final response = await _dio.get('$_baseUrl/doctor_rooms');
      return (response.data['doctor_rooms']) as List<dynamic>;
    } on DioException catch (e) {
      throw _formatError(e);
    }
  }

  /// Назначить врача на кабинет
  Future<Map<String, dynamic>> assignDoctorToRoom(
      int roomNumber, int doctorId) async {
    try {
      final response = await _dio.post(
        '$_baseUrl/doctor_rooms/$roomNumber/assign',
        data: {'doctor_id': doctorId},
      );
      return response.data as Map<String, dynamic>;
    } on DioException catch (e) {
      throw _formatError(e);
    }
  }

  /// Снять врача с кабинета
  Future<Map<String, dynamic>> unassignDoctorFromRoom(
      int roomNumber, int doctorId) async {
    try {
      final response = await _dio.post(
        '$_baseUrl/doctor_rooms/$roomNumber/unassign',
        data: {'doctor_id': doctorId},
      );
      return response.data as Map<String, dynamic>;
    } on DioException catch (e) {
      throw _formatError(e);
    }
  }

  String _formatError(DioException e) {
    return e.response != null
        ? 'Ошибка ${e.response?.statusCode}: ${e.response?.data}'
        : 'Сетевая ошибка: ${e.message}';
  }
}
