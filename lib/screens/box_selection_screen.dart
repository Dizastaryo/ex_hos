import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import '../providers/box_provider.dart';
import '../models/box_model.dart';
import 'package:model_viewer_plus/model_viewer_plus.dart';
import 'box_screen.dart'; // Импортируем страницу BoxScreen

class BoxSelectionScreen extends StatefulWidget {
  const BoxSelectionScreen({super.key});

  @override
  State<BoxSelectionScreen> createState() => _BoxSelectionScreenState();
}

class _BoxSelectionScreenState extends State<BoxSelectionScreen>
    with TickerProviderStateMixin {
  BoxModel? selectedBox;
  late TabController _tabController;

  @override
  void initState() {
    super.initState();
    _tabController = TabController(length: 6, vsync: this);
  }

  @override
  void dispose() {
    _tabController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final boxes = Provider.of<BoxProvider>(context).boxes;

    return DefaultTabController(
      length: 6,
      child: Scaffold(
        appBar: AppBar(
          title: const Text('Карта склада'),
          backgroundColor: const Color(0xFF6C9942),
          bottom: TabBar(
            controller: _tabController,
            tabs: const [
              Tab(text: 'XXS'),
              Tab(text: 'XS'),
              Tab(text: 'S'),
              Tab(text: 'M'),
              Tab(text: 'L'),
              Tab(text: 'XL'),
            ],
            onTap: (index) {
              setState(() {
                selectedBox = null;
              });
            },
          ),
        ),
        body: Column(
          children: [
            Expanded(
              flex: 1,
              child: TabBarView(
                controller: _tabController,
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
            // Сцена с 3D моделью или сообщением
            Flexible(
              flex: 1,
              child: Padding(
                padding: const EdgeInsets.symmetric(
                    horizontal: 16.0, vertical: 40.0),
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
                  child: selectedBox != null
                      ? ModelViewer(
                          src:
                              "assets/3d_models/${selectedBox!.type.name.toUpperCase()}.glb",
                          autoRotate: true,
                          cameraControls: true,
                          alt:
                              "3D модель ${selectedBox!.type.name.toUpperCase()}")
                      : Center(
                          child: Text(
                            'Выберите бокс',
                            style:
                                TextStyle(fontSize: 18, color: Colors.black54),
                          ),
                        ),
                ),
              ),
            ),
            // Кнопка "Арендовать" только если коробка выбрана
            if (selectedBox != null)
              Padding(
                padding: const EdgeInsets.all(16.0),
                child: ElevatedButton(
                  onPressed: () {
                    // Переход на страницу BoxScreen
                    Navigator.push(
                      context,
                      MaterialPageRoute(
                        builder: (context) => BoxScreen(
                            box: selectedBox!), // Передаем выбранную коробку
                      ),
                    );
                  },
                  child: const Text('Арендовать'),
                  style: ElevatedButton.styleFrom(
                    backgroundColor: const Color(0xFF6C9942),
                    padding: const EdgeInsets.symmetric(vertical: 16.0),
                    textStyle: const TextStyle(
                      fontSize: 16,
                      fontWeight: FontWeight.bold,
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
                    selectedBox = box;
                  });
                }
              },
              child: Container(
                decoration: BoxDecoration(
                  color: box.isAvailable ? const Color(0xFF6C9942) : Colors.red,
                  borderRadius: BorderRadius.circular(8),
                  border: selectedBox == box
                      ? Border.all(color: Colors.blue, width: 3)
                      : null,
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
