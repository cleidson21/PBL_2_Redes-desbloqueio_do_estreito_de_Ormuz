echo "🔨 Construindo as imagens do PBL 2..."

docker build -t cleidsonramos/servidor:latest ./servidor
docker build -t cleidsonramos/dashboard:latest ./dashboard
docker build -t cleidsonramos/sensor_tlm:latest ./sensor_tlm
docker build -t cleidsonramos/radar_tcp:latest ./radar_tcp
docker build -t cleidsonramos/drone:latest ./drone

echo "✅ Build concluído!"