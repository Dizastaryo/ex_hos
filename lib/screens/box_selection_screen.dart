import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import '../providers/box_provider.dart';
import '../models/box_model.dart';
import 'package:model_viewer_plus/model_viewer_plus.dart';

class BoxSelectionScreen extends StatefulWidget {
  const BoxSelectionScreen({super.key});

  @override
  State<BoxSelectionScreen> createState() => _BoxSelectionScreenState();
}

class _BoxSelectionScreenState extends State<BoxSelectionScreen> {
  BoxModel? selectedBox; // Переменная для хранения выбранной коробки

  @override
  Widget build(BuildContext context) {
    final boxes = Provider.of<BoxProvider>(context).boxes;

    return DefaultTabController(
      length: 6,
      child: Scaffold(
        appBar: AppBar(
          title: const Text('Карта склада'),
          backgroundColor: const Color(0xFF6C9942),
          bottom: const TabBar(
            tabs: [
              Tab(text: 'XXS'),
              Tab(text: 'XS'),
              Tab(text: 'S'),
              Tab(text: 'M'),
              Tab(text: 'L'),
              Tab(text: 'XL'),
            ],
          ),
        ),
        body: Column(
          children: [
            Expanded(
              flex: 1,
              child: TabBarView(
                children: [
                  _buildBoxGrid(context, boxes, BoxType.xxs),
                  _buildBoxGrid(context, boxes, BoxType.xs),
                  _buildBoxGrid(context, boxes, BoxType.s),
                  _buildBoxGrid(context, boxes, BoxType.m),
                  _buildBoxGrid(context, boxes, BoxType.l),
                  _buildBoxGrid(context, boxes, BoxType.xl),
                ],
              ),
            ),
            if (selectedBox !=
                null) // Показываем 3D модель только если выбрана коробка
              Expanded(
                flex: 1,
                child: Padding(
                  padding: const EdgeInsets.all(16.0),
                  child: Container(
                    height: 300,
                    decoration: BoxDecoration(
                      borderRadius: BorderRadius.circular(16),
                      boxShadow: const [
                        BoxShadow(
                          color: Colors.black26,
                          blurRadius: 10,
                          offset: Offset(0, 4),
                        ),
                      ],
                    ),
                    child: ModelViewer(
                      src:
                          "assets/3d_models/${selectedBox!.type.name.toUpperCase()}.glb",
                      autoRotate: true,
                      cameraControls: true,
                      alt: "3D модель ${selectedBox!.type.name.toUpperCase()}",
                    ),
                  ),
                ),
              ),
          ],
        ),
      ),
    );
  }

  Widget _buildBoxGrid(
      BuildContext context, List<BoxModel> boxes, BoxType type) {
    final filteredBoxes = boxes.where((box) => box.type == type).toList();

    return Padding(
      padding: const EdgeInsets.all(16),
      child: InteractiveViewer(
        boundaryMargin: const EdgeInsets.all(20),
        minScale: 0.5,
        maxScale: 2.0,
        child: GridView.builder(
          gridDelegate: const SliverGridDelegateWithFixedCrossAxisCount(
            crossAxisCount: 6,
            crossAxisSpacing: 8.0,
            mainAxisSpacing: 8.0,
          ),
          itemCount: filteredBoxes.length,
          itemBuilder: (context, index) {
            final box = filteredBoxes[index];
            return GestureDetector(
              onTap: () {
                if (box.isAvailable) {
                  setState(() {
                    selectedBox = box; // Обновляем выбранную коробку
                  });
                }
              },
              child: Container(
                decoration: BoxDecoration(
                  color: box.isAvailable ? const Color(0xFF6C9942) : Colors.red,
                  borderRadius: BorderRadius.circular(8),
                ),
                child: Center(
                  child: Text(
                    box.id,
                    style: const TextStyle(
                      color: Colors.white,
                      fontWeight: FontWeight.bold,
                    ),
                  ),
                ),
              ),
            );
          },
        ),
      ),
    );
  }
}
