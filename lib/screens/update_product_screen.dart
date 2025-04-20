import 'dart:io';
import 'package:flutter/material.dart';
import 'package:dio/dio.dart';
import 'package:image_picker/image_picker.dart';
import 'package:provider/provider.dart';
import '../models/product.dart';
import '../models/category.dart';
import '../services/product_service.dart';
import '../services/category_service.dart';

class UpdateProductScreen extends StatefulWidget {
  final int productId;
  final Product product;

  const UpdateProductScreen({
    required this.productId,
    required this.product,
    Key? key,
  }) : super(key: key);

  @override
  _UpdateProductScreenState createState() => _UpdateProductScreenState();
}

class _UpdateProductScreenState extends State<UpdateProductScreen> {
  final _formKey = GlobalKey<FormState>();
  final _nameController = TextEditingController();
  final _descriptionController = TextEditingController();
  final _priceController = TextEditingController();
  final _picker = ImagePicker();

  List<File> _imageFiles = [];
  List<Category> _categories = [];
  int? _selectedCategoryId;

  @override
  void initState() {
    super.initState();
    _initializeData();
    _loadCategories();
  }

  void _initializeData() {
    _nameController.text = widget.product.name;
    _descriptionController.text = widget.product.description;
    _priceController.text = widget.product.price.toString();
    _selectedCategoryId = widget.product.categoryId;
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
    if (!_formKey.currentState!.validate() || _selectedCategoryId == null)
      return;

    final updatedProduct = ProductCreate(
      name: _nameController.text.trim(),
      description: _descriptionController.text.trim(),
      price: double.parse(_priceController.text.trim()),
      categoryId: _selectedCategoryId!,
    );

    try {
      final service = context.read<ProductService>();
      await service.updateProduct(
        id: widget.productId,
        product: updatedProduct,
        images: _imageFiles,
      );
      Navigator.pop(context);
    } catch (e) {
      _showError('Ошибка обновления продукта: $e');
    }
  }

  void _showError(String message) {
    ScaffoldMessenger.of(context)
        .showSnackBar(SnackBar(content: Text(message)));
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
            Image.file(file,
                fit: BoxFit.cover,
                width: double.infinity,
                height: double.infinity),
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
      appBar: AppBar(title: const Text('Редактировать продукт')),
      body: Padding(
        padding: const EdgeInsets.all(16.0),
        child: Form(
          key: _formKey,
          child: ListView(
            children: [
              TextFormField(
                controller: _nameController,
                decoration: const InputDecoration(labelText: 'Название'),
                validator: (v) =>
                    v == null || v.isEmpty ? 'Обязательное поле' : null,
              ),
              TextFormField(
                controller: _descriptionController,
                decoration: const InputDecoration(labelText: 'Описание'),
                validator: (v) =>
                    v == null || v.isEmpty ? 'Обязательное поле' : null,
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
              const SizedBox(height: 16),
              DropdownButtonFormField<int>(
                value: _selectedCategoryId,
                decoration: const InputDecoration(labelText: 'Категория'),
                items: _categories
                    .map((c) => DropdownMenuItem(
                          value: c.id,
                          child: Text(c.name),
                        ))
                    .toList(),
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
                const Text('Новые изображения:'),
                _buildImagePreview(),
              ],
              const SizedBox(height: 24),
              ElevatedButton(
                onPressed: _submitForm,
                child: const Text('Сохранить изменения'),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
