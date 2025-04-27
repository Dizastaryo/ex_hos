import 'package:dio/dio.dart';
import '../models/category.dart';
import '../models/profile.dart';

class CategoryService {
  final Dio _dio;

  // Принимаем Dio через конструктор
  CategoryService(this._dio);

  Future<List<Category>> getCategories() async {
    final response = await _dio.get('https://172.20.10.2:8000/categories/');
    return (response.data as List)
        .map((json) => Category.fromJson(json))
        .toList();
  }

  Future<Profile> getProfile() async {
    final response =
        await _dio.get('https://172.20.10.2:8000/categories/profile');
    return Profile.fromJson(response.data);
  }
}
