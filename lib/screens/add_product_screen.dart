import 'dart:io';
import 'package:flutter/material.dart';
import 'package:image_picker/image_picker.dart';
import 'package:provider/provider.dart';

import '../services/product_service.dart';
import '../services/category_service.dart';
import '../models/product.dart';
import '../models/category.dart';

class AddProductScreen extends StatefulWidget {
  const AddProductScreen({super.key});

  @override
  State<AddProductScreen> createState() => _AddProductScreenState();
}

class _AddProductScreenState extends State<AddProductScreen> {
  final _formKey = GlobalKey<FormState>();
  final _nameController = TextEditingController();
  final _descriptionController = TextEditingController();
  final _priceController = TextEditingController();
  final _picker = ImagePicker();

  int? _selectedCategoryId;
  List<Category> _categories = [];
  List<File> _imageFiles = [];

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => _loadCategories());
  }

  Future<void> _loadCategories() async {
    try {
      final service = context.read<CategoryService>();
      final result = await service.getCategories();
      setState(() => _categories = result);
    } catch (e) {
      _showError('Ошибка загрузки категорий: $e');
    }
  }

  Future<void> _pickImages() async {
    try {
      final picked = await _picker.pickMultiImage(
        maxWidth: 1024,
        maxHeight: 1024,
        imageQuality: 85,
      );
      if (picked != null) {
        setState(() {
          _imageFiles.addAll(picked.map((x) => File(x.path)));
        });
      }
    } catch (e) {
      _showError('Ошибка при выборе изображений: $e');
    }
  }

  Future<void> _submitForm() async {
    if (!_formKey.currentState!.validate() || _selectedCategoryId == null) return;

    final product = ProductCreate(
      name: _nameController.text.trim(),
      description: _descriptionController.text.trim(),
      price: double.parse(_priceController.text.trim()),
      categoryId: _selectedCategoryId!,
    );

    try {
      final service = context.read<ProductService>();
      await service.addProduct(product: product, images: _imageFiles);
      Navigator.pop(context, true);
    } catch (e) {
      _showError('Ошибка создания продукта: $e');
    }
  }

  void _showError(String message) {
    ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(message)));
  }

  Widget _buildImagePreview() {
    return GridView.builder(
      shrinkWrap: true,
      physics: const NeverScrollableScrollPhysics(),
      gridDelegate: const SliverGridDelegateWithFixedCrossAxisCount(
        crossAxisCount: 3,
        crossAxisSpacing: 4,
        mainAxisSpacing: 4,
      ),
      itemCount: _imageFiles.length,
      itemBuilder: (ctx, index) {
        final file = _imageFiles[index];
        return Stack(
          children: [
            Image.file(file, fit: BoxFit.cover, width: double.infinity, height: double.infinity),
            Positioned(
              right: 0,
              child: IconButton(
                icon: const Icon(Icons.close, color: Colors.red),
                onPressed: () => setState(() => _imageFiles.removeAt(index)),
              ),
            ),
          ],
        );
      },
    );
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('Добавить продукт')),
      body: Padding(
        padding: const EdgeInsets.all(16.0),
        child: Form(
          key: _formKey,
          child: ListView(
            children: [
              TextFormField(
                controller: _nameController,
                decoration: const InputDecoration(labelText: 'Название'),
                validator: (v) => v == null || v.isEmpty ? 'Обязательное поле' : null,
              ),
              TextFormField(
                controller: _descriptionController,
                decoration: const InputDecoration(labelText: 'Описание'),
                validator: (v) => v == null || v.isEmpty ? 'Обязательное поле' : null,
              ),
              TextFormField(
                controller: _priceController,
                decoration: const InputDecoration(labelText: 'Цена'),
                keyboardType: TextInputType.number,
                validator: (v) {
                  final value = double.tryParse(v ?? '');
                  return value == null ? 'Некорректная цена' : null;
                },
              ),
              DropdownButtonFormField<int>(
                value: _selectedCategoryId,
                decoration: const InputDecoration(labelText: 'Категория'),
                items: _categories.map((c) => DropdownMenuItem(value: c.id, child: Text(c.name))).toList(),
                onChanged: (v) => setState(() => _selectedCategoryId = v),
                validator: (v) => v == null ? 'Выберите категорию' : null,
              ),
              const SizedBox(height: 16),
              ElevatedButton.icon(
                icon: const Icon(Icons.photo_library),
                label: const Text('Добавить изображения'),
                onPressed: _pickImages,
              ),
              if (_imageFiles.isNotEmpty) ...[
                const SizedBox(height: 16),
                const Text('Выбранные изображения:'),
                _buildImagePreview(),
              ],
              const SizedBox(height: 24),
              ElevatedButton(
                onPressed: _submitForm,
                child: const Text('Создать продукт'),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
