const { createApp, ref, onMounted, computed, watch, nextTick } = Vue;

createApp({
    setup() {
        const view = ref('landing'); // Default to landing
        const adminTab = ref('players');
        const players = ref([]);
        const flights = ref([]);
        const results = ref([]);
        const course = ref([]);
        const newFlightName = ref('');
        const newFlightStartingHole = ref(1);
        const flightToken = ref('');
        const currentFlight = ref(null);
        const scores = ref({}); // Map of playerID -> hole -> strokes
        const showWarning = ref(false);
        const warningMessage = ref('');

        // Player Form State
        const playerForm = ref({ id: 0, name: '', surname: '', reg_num: '', handicap: 0, gender: 'M' });
        const isEditing = ref(false);
        const isFetchingHCP = ref(false);

        // Fetch Players
        const fetchPlayers = async () => {
            const res = await fetch('/api/players');
            players.value = await res.json();
        };

        // Fetch Course
        const fetchCourse = async () => {
            const res = await fetch('/api/course');
            course.value = await res.json();
        };

        // Save Course
        const saveCourse = async () => {
            await fetch('/api/course', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(course.value)
            });
            alert('Course saved!');
        };

        // Fetch Flights
        const fetchFlights = async () => {
            const res = await fetch('/api/flights');
            flights.value = await res.json();
            setupDragAndDrop();
        };

        // Fetch Results
        const fetchResults = async () => {
            const res = await fetch('/api/results');
            results.value = await res.json();
        };

        // Upload Players
        const uploadPlayers = async (event) => {
            const file = event.target.files[0];
            if (!file) return;
            const formData = new FormData();
            formData.append('file', file);
            await fetch('/api/players/import', {
                method: 'POST',
                body: formData
            });
            fetchPlayers();
        };

        const uploadCourse = async (event) => {
            const file = event.target.files[0];
            if (!file) return;
            const formData = new FormData();
            formData.append('file', file);
            await fetch('/api/course/import', {
                method: 'POST',
                body: formData
            });
            fetchCourse();
        };

        // Save Player (Create or Update)
        const savePlayer = async () => {
            if (!playerForm.value.name || !playerForm.value.surname) {
                alert('Name and Surname are required');
                return;
            }
            await fetch('/api/players', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(playerForm.value)
            });
            cancelEdit();
            fetchPlayers();
        };

        const editPlayer = (player) => {
            playerForm.value = { ...player };
            isEditing.value = true;
        };

        const cancelEdit = () => {
            playerForm.value = { id: 0, name: '', surname: '', reg_num: '', handicap: 0, gender: 'M' };
            isEditing.value = false;
        };

        const fetchHCPs = async () => {
            isFetchingHCP.value = true;
            try {
                await fetch('/api/players/fetch-hcp', { method: 'POST' });
                await fetchPlayers();
            } catch (e) {
                console.error(e);
            } finally {
                isFetchingHCP.value = false;
            }
        };

        // Delete Player
        const deletePlayer = async (id) => {
            if (!confirm('Are you sure?')) return;
            await fetch('/api/players/delete', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ id })
            });
            fetchPlayers();
        };

        // Create Flight
        const createFlight = async () => {
            if (!newFlightName.value) return;
            await fetch('/api/flights', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    name: newFlightName.value,
                    starting_hole: newFlightStartingHole.value
                })
            });
            newFlightName.value = '';
            newFlightStartingHole.value = 1;
            fetchFlights();
        };

        // Update Flight (Name or Starting Hole)
        const updateFlight = async (flight) => {
            await fetch('/api/flights/update', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    id: flight.id,
                    name: flight.name,
                    starting_hole: flight.starting_hole
                })
            });
            fetchFlights();
        };

        const randomAssign = async () => {
            if (!confirm('Tato akce náhodně přiřadí všechny zbylé hráče do neobsazených míst ve flightech. Pokračovat?')) return;
            await fetch('/api/flights/random-assign', { method: 'POST' });
            fetchFlights();
        };
        // Delete Flight
        const deleteFlight = async (id) => {
            // if (!confirm('Are you sure you want to delete this flight? Players will be unassigned.')) return;
            await fetch('/api/flights', {
                method: 'DELETE',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ id })
            });
            fetchFlights();
            fetchPlayers(); // To update unassigned list
        };

        // Computed Unassigned Players
        const unassignedPlayers = computed(() => {
            const assignedIds = new Set();
            if (flights.value) {
                flights.value.forEach(f => {
                    if (f.players) {
                        f.players.forEach(p => assignedIds.add(p.id));
                    }
                });
            }
            return players.value.filter(p => !assignedIds.has(p.id));
        });

        // Drag and Drop Setup
        const setupDragAndDrop = () => {
            // Use nextTick to ensure DOM is updated
            Vue.nextTick(() => {
                const unassignedEl = document.getElementById('unassigned-players');
                if (unassignedEl) {
                    // Destroy existing instance if any (not tracking them currently, but Sortable handles re-init gracefully usually, 
                    // but better to be safe if we were tracking. For now just new Sortable)
                    new Sortable(unassignedEl, {
                        group: 'players',
                        animation: 150,
                        onAdd: async (evt) => {
                            const playerId = parseInt(evt.item.getAttribute('data-id'));
                            await unassignPlayer(playerId);
                        }
                    });
                }

                if (flights.value) {
                    flights.value.forEach(f => {
                        const el = document.getElementById('flight-' + f.id);
                        if (el) {
                            new Sortable(el, {
                                group: 'players',
                                animation: 150,
                                onAdd: async (evt) => {
                                    const playerId = parseInt(evt.item.getAttribute('data-id'));
                                    const flightId = parseInt(evt.to.getAttribute('data-flight-id'));
                                    await assignPlayer(flightId, playerId);
                                }
                            });
                        }
                    });
                }
            });
        };

        const assignPlayer = async (flightId, playerId) => {
            await fetch('/api/flights/assign', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ flight_id: flightId, player_id: playerId })
            });
            // Refresh to ensure state is consistent
            fetchFlights();
            fetchPlayers();
        };

        const unassignPlayer = async (playerId) => {
            await fetch('/api/flights/unassign', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ player_id: playerId })
            });
            fetchFlights();
            fetchPlayers();
        };

        // Scoring Logic
        const loadFlight = async () => {
            // In a real app, we'd search by token. For now, let's just find the flight in the list
            // But the API doesn't support search by token yet. 
            // Let's just fetch all flights and filter client side for this prototype
            await fetchFlights();
            currentFlight.value = flights.value.find(f => f.token === flightToken.value);
            if (!currentFlight.value) {
                alert('Flight not found');
                return;
            }

            // Fetch scores for all players in flight
            for (const player of currentFlight.value.players) {
                const res = await fetch(`/api/scores?player_id=${player.id}`);
                const playerScores = await res.json();
                for (const [hole, strokes] of Object.entries(playerScores)) {
                    scores.value[`${player.id}-${hole}`] = strokes;
                }
            }
        };

        const getScore = (playerId, hole) => {
            return scores.value[`${playerId}-${hole}`] || '';
        };

        const submitScore = async (playerId, hole, strokes) => {
            scores.value[`${playerId}-${hole}`] = strokes;
            await fetch('/api/scores', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    player_id: playerId,
                    hole_number: hole,
                    strokes: parseInt(strokes)
                })
            });
        };

        // ... (inside setup)

        const getDistance = (hole, player) => {
            const h = (course.value && course.value[hole - 1]);
            if (!h) return 0;
            if (player && player.gender === 'F') return h.length_red;
            return h.length_yellow;
        };


        // ... (inside setup)

        const generateQRs = () => {
            nextTick(() => {
                flights.value.forEach(flight => {
                    const container = document.getElementById('qr-' + flight.id);
                    if (container) {
                        container.innerHTML = '';
                        new QRCode(container, {
                            text: `https://vanoce.jdark.org/?t=${flight.token}`,
                            width: 150,
                            height: 150,
                            colorDark: "#000000",
                            colorLight: "#ffffff",
                            correctLevel: QRCode.CorrectLevel.L
                        });
                    }
                });
            });
        };

        const downloadQR = (flight) => {
            const container = document.getElementById('qr-' + flight.id);
            if (!container) return;
            const img = container.querySelector('img');
            if (!img) return;

            const link = document.createElement('a');
            link.href = img.src;
            link.download = `${flight.name}_${flight.token}.png`;
            document.body.appendChild(link);
            link.click();
            document.body.removeChild(link);
        };

        const downloadAllQRs = () => {
            flights.value.forEach((flight, index) => {
                setTimeout(() => {
                    downloadQR(flight);
                }, index * 200); // Stagger downloads to avoid browser blocking
            });
        };

        watch(adminTab, (newVal) => {
            if (newVal === 'flights') {
                // Fetch flights again to be sure we have latest data and then setup DnD
                fetchFlights();
            } else if (newVal === 'flights-qr') {
                generateQRs();
            }
        });

        onMounted(() => {
            fetchPlayers();
            fetchFlights();
            fetchResults();
            fetchCourse();

            const path = window.location.pathname;
            const urlParams = new URLSearchParams(window.location.search);
            const tokenParam = urlParams.get('token') || urlParams.get('t');

            if (path === '/adminpage') {
                view.value = 'admin';
            } else if (tokenParam) {
                view.value = 'scoring';
                flightToken.value = tokenParam;
                loadFlight();
            } else {
                view.value = 'landing';
            }
        });

        // Helper: Get Initials
        const getInitials = (name, surname) => {
            return (name.charAt(0) + surname.charAt(0)).toUpperCase();
        };

        // Multi-Picker State
        const showPicker = ref(false);
        const pickerHole = ref(0);
        const pickerValues = ref({}); // { playerId: score }
        const pickerRefs = {}; // { playerId: element }

        const setPickerRef = (playerId, el) => {
            if (el) pickerRefs[playerId] = el;
        };

        const openPicker = (hole) => {
            const startHole = currentFlight.value.starting_hole || 1;

            // If this is NOT the starting hole, check if starting hole has at least one score
            if (hole !== startHole) {
                const hasStartScore = currentFlight.value.players.some(p => getScore(p.id, startHole) !== '');
                if (!hasStartScore) {
                    warningMessage.value = `Omlouváme se, dokud není zapsán výsledek na vaší startovní jamce (č. ${startHole}), nelze zapisovat na ostatní jamky.`;
                    showWarning.value = true;
                    return;
                }
            }

            pickerHole.value = hole;
            const par = getPar(hole);

            // Create a new object for reactivity
            const newValues = {};
            currentFlight.value.players.forEach(p => {
                const s = getScore(p.id, hole);
                newValues[p.id] = s ? parseInt(s) : par;
            });
            pickerValues.value = newValues;

            showPicker.value = true;

            // Scroll each picker to its value after full render
            nextTick(() => {
                // Short timeout to ensure flex/box layout is fully computed
                setTimeout(() => {
                    Object.entries(pickerValues.value).forEach(([playerId, val]) => {
                        scrollToPlayerValue(parseInt(playerId), val);
                    });
                }, 50);
            });
        };

        const scrollToPlayerValue = (playerId, val) => {
            pickerValues.value[playerId] = val;
            const el = pickerRefs[playerId];
            if (el) {
                const itemHeight = 40;
                el.scrollTop = val * itemHeight;
            }
        };

        const onPlayerScroll = (playerId, e) => {
            const itemHeight = 40;
            const scrollTop = e.target.scrollTop;
            const index = Math.round(scrollTop / itemHeight);
            pickerValues.value[playerId] = index;
        };

        const confirmScore = async () => {
            // Submit scores for all players
            const promises = Object.entries(pickerValues.value).map(([playerId, val]) => {
                return submitScore(parseInt(playerId), pickerHole.value, val);
            });
            await Promise.all(promises);
            showPicker.value = false;
        };

        const getPar = (hole) => {
            return (course.value && course.value[hole - 1]) ? course.value[hole - 1].par : 4;
        };

        const getScoreStyle = (score, hole) => {
            if (!score) return { backgroundColor: '#E6E6E6' }; // Default gray

            const par = getPar(hole);
            const diff = parseInt(score) - par;

            // Premium Golf Color Palette
            if (diff === 0) return { backgroundColor: '#FCF6A1' }; // Par - Soft Yellow
            if (diff === 1) return { backgroundColor: '#D3D1EB' }; // Bogey - Lavender
            if (diff === 2) return { backgroundColor: '#BBB0EB' }; // Double Bogey - Purple
            if (diff >= 3) return { backgroundColor: '#9A78DB' }; // Triple+ - Deep Purple
            if (diff === -1) return { backgroundColor: '#F5C5C4' }; // Birdie - Soft Red/Salmon
            if (diff <= -2) return { backgroundColor: '#F59391' }; // Eagle+ - Vibrant Salmon

            return { backgroundColor: '#E6E6E6' };
        };

        const getPlayerTotal = (playerId, startHole, endHole) => {
            let total = 0;
            for (let h = startHole; h <= endHole; h++) {
                const s = scores.value[`${playerId}-${h}`];
                if (s) total += parseInt(s);
            }
            return total;
        };

        const isHoleScored = (hole) => {
            if (!currentFlight.value) return false;
            return currentFlight.value.players.some(p => getScore(p.id, hole) !== '');
        };

        const closeWarning = () => {
            showWarning.value = false;
        };

        const closePicker = () => {
            showPicker.value = false;
        };

        return {
            view,
            adminTab,
            players,
            flights,
            results,
            course,
            saveCourse,
            newFlightName,
            newFlightStartingHole,
            flightToken,
            currentFlight,
            unassignedPlayers,
            playerForm,
            isEditing,
            savePlayer,
            editPlayer,
            cancelEdit,
            uploadPlayers,
            uploadCourse,
            deletePlayer,
            createFlight,
            updateFlight,
            deleteFlight,
            loadFlight,
            getScore,
            submitScore,
            getInitials,
            showPicker,
            showWarning,
            warningMessage,
            pickerValues,
            pickerHole,
            setPickerRef,
            openPicker,
            scrollToPlayerValue,
            onPlayerScroll,
            confirmScore,
            closeWarning,
            closePicker,
            getDistance,
            getPar,
            getPlayerTotal,
            getScoreStyle,
            isHoleScored,
            downloadQR,
            downloadAllQRs,
            randomAssign,
            isFetchingHCP,
            fetchHCPs
        };
    }
}).mount('#app');
